package pkg

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chainguard-dev/clog"
	"github.com/chainguard-dev/gopom"
	"github.com/ghodss/yaml"
)

/* Example patch for 'proper' dependency:
<dependency>
  <groupId>io.projectreactor.netty</groupId>
  <artifactId>reactor-netty-http</artifactId>
  <version>1.0.39</version>
</dependency>
*/

type PatchList struct {
	Patches []Patch `json:"patches"`
}

// Should this just be a gopom.Dependency??
// Just start with this for now, change to it if need arises.
// For now, this is easier to read since the upstream is
// xml based, no other real reason.
type Patch struct {
	GroupID    string `json:"groupId" yaml:"groupId"`
	ArtifactID string `json:"artifactId" yaml:"artifactId"`
	Version    string `json:"version" yaml:"version"`
	Scope      string `json:"scope,omitempty" yaml:"scope,omitempty"`
	Type       string `json:"type,omitempty" yaml:"type,omitempty"`
}

type PropertyList struct {
	Properties []PropertyPatch `json:"properties" yaml:"properties"`
}

/*
<!-- dependency versions -->
<slf4j.version>1.7.30</slf4j.version>
-    <logback-version>1.2.10</logback-version>
+    <logback-version>1.2.13</logback-version>
*/
// These are just map[string]string and just a blind overwrite.
type PropertyPatch struct {
	Property string `json:"property" yaml:"property"`
	Value    string `json:"value" yaml:"value"`
}

// Default scope and type for a dependency. Are these even right?
const (
	defaultScope = "import"
	defaultType  = "jar"
)

// PatchProject will update versions for all matched dependencies
// if they are found in Project.Dependencies. If there is no
// match, it will add the dependency to the project.
// Also does a blind overwrite of any properties with propertyPatches.
// TODO(vaikas): Figure out when / if to use DependencyManagement instead.
func PatchProject(ctx context.Context, project *gopom.Project, patches []Patch, propertyPatches map[string]string) (*gopom.Project, error) {
	log := clog.FromContext(ctx)
	if project == nil {
		return nil, fmt.Errorf("project is nil")
	}
	// If there are no straight up version replacements, but
	// for some reason a dependency is missing, gather them here
	// so that we can add them later.
	missingDeps := make(map[Patch]Patch)
	for _, p := range patches {
		log.Infof("Have patch: %s.%s:%s", p.GroupID, p.ArtifactID, p.Version)
		missingDeps[p] = p
	}

	// If there are any hard coded dependencies that need to be patched, do
	// that here.
	// Note that we do not patch scope, or type, since they should already be
	// configured correctly.
	if project.Dependencies != nil {
		for i, dep := range *project.Dependencies {
			log.Infof("Checking DEP: %s.%s:%s", dep.GroupID, dep.ArtifactID, dep.Version)
			for _, patch := range patches {
				if dep.ArtifactID == patch.ArtifactID &&
					dep.GroupID == patch.GroupID {
					log.Infof("Patching %s.%s from %s to %s with scope: %s", patch.GroupID, patch.ArtifactID, dep.Version, patch.Version, patch.Scope)
					(*project.Dependencies)[i].Version = patch.Version

					// Found it, so remove it from the missing deps
					// This is dump, make it better.
					delete(missingDeps, patch)
				}
			}
		}
	}

	if project.Dependencies != nil {
		for _, dep := range *project.Dependencies {
			log.Debugf("DEP AFTER patching: %s.%s:%s", dep.GroupID, dep.ArtifactID, dep.Version)
		}
	}

	// Note that we do not patch scope, or type, since they should already be
	// configured correctly.
	if project.DependencyManagement != nil {
		for i, dep := range *project.DependencyManagement.Dependencies {
			log.Debugf("Checking DM DEP: %s.%s:%s", dep.GroupID, dep.ArtifactID, dep.Version)
			for _, patch := range patches {
				if dep.ArtifactID == patch.ArtifactID &&
					dep.GroupID == patch.GroupID {
					log.Infof("Patching DM dep %s.%s from %s to %s with scope: %s", patch.GroupID, patch.ArtifactID, dep.Version, patch.Version, patch.Scope)
					(*project.DependencyManagement.Dependencies)[i].Version = patch.Version
					// Found it, so remove it from the missing deps
					// This is dump, make it better.
					delete(missingDeps, patch)
				}
			}
		}
	}

	// If there are any missing dependencies, add them in. I guess add them
	// to DependencyManagement?
	if project.DependencyManagement == nil && len(missingDeps) > 0 {
		project.DependencyManagement = &gopom.DependencyManagement{}
	}
	for md := range missingDeps {
		md := md
		log.Infof("Adding missing dependency: %s.%s:%s", md.GroupID, md.ArtifactID, md.Version)

		*project.DependencyManagement.Dependencies = append(*project.DependencyManagement.Dependencies, gopom.Dependency{
			GroupID:    md.GroupID,
			ArtifactID: md.ArtifactID,
			Version:    md.Version,
			Scope:      md.Scope,
			Type:       md.Type,
		})
	}
	if project.Properties == nil && len(propertyPatches) > 0 {
		project.Properties = &gopom.Properties{Entries: propertyPatches}
	} else {
		for k, v := range propertyPatches {
			val, exists := project.Properties.Entries[k]
			if exists {
				log.Infof("Patching property: %s from %s to %s", k, val, v)
			} else {
				log.Infof("Creating property: %s as %s", k, v)
			}
			project.Properties.Entries[k] = v
		}
	}
	return project, nil
}

func ParsePatches(patchFile, patchFlag string) ([]Patch, error) {
	if patchFile != "" {
		var patchList PatchList
		file, err := os.Open(patchFile)
		if err != nil {
			return nil, fmt.Errorf("failed reading file: %w", err)
		}
		defer file.Close()
		byteValue, _ := io.ReadAll(file)
		if err := yaml.Unmarshal(byteValue, &patchList); err != nil {
			return nil, err
		}
		for i := range patchList.Patches {
			if patchList.Patches[i].Scope == "" {
				patchList.Patches[i].Scope = defaultScope
			}
			if patchList.Patches[i].Type == "" {
				patchList.Patches[i].Type = defaultType
			}
		}
		return patchList.Patches, nil
	}
	dependencies := strings.Split(patchFlag, " ")
	patches := []Patch{}
	for _, dep := range dependencies {
		if dep == "" {
			continue
		}
		parts := strings.Split(dep, "@")
		if len(parts) < 3 {
			return nil, fmt.Errorf("invalid dependencies format (%s). Each dependency should be in the format <groupID@artifactID@version[@scope]>. Usage: pombump --dependencies=\"<groupID@artifactID@version@scope> <groupID@artifactID@version> ...\"", dep)
		}
		// Default scope. Maybe make this configurable?
		scope := defaultScope
		if len(parts) >= 4 {
			scope = parts[3]
		}
		depType := defaultType
		if len(parts) >= 5 {
			depType = parts[4]
		}
		patches = append(patches, Patch{GroupID: parts[0], ArtifactID: parts[1], Version: parts[2], Scope: scope, Type: depType})
	}
	return patches, nil
}

func ParseProperties(propertyFile, propertiesFlag string) (map[string]string, error) {
	propertiesPatches := map[string]string{}
	if propertyFile != "" {
		var propertyList PropertyList
		file, err := os.Open(propertyFile)
		if err != nil {
			return nil, fmt.Errorf("failed reading file: %w", err)
		}
		defer file.Close()
		byteValue, _ := io.ReadAll(file)
		if err := yaml.Unmarshal(byteValue, &propertyList); err != nil {
			return nil, err
		}
		for _, v := range propertyList.Properties {
			propertiesPatches[v.Property] = v.Value
		}
		return propertiesPatches, nil
	}

	properties := strings.Split(propertiesFlag, " ")
	for _, prop := range properties {
		if prop == "" {
			continue
		}
		parts := strings.Split(prop, "@")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid properties format. Each dependency should be in the format <property@value>. Usage: pombump --properties=\"<property@value> <property@value>\" ...\"")
		}
		propertiesPatches[parts[0]] = parts[1]
	}

	return propertiesPatches, nil
}
