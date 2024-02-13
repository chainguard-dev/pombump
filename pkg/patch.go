package pkg

import (
	"fmt"

	"github.com/2000Slash/gopom"
)

/* Example patch for 'proper' dependency:
<dependency>
  <groupId>io.projectreactor.netty</groupId>
  <artifactId>reactor-netty-http</artifactId>
  <version>1.0.39</version>
</dependency>
*/

// Should this just be a gopom.Dependency??
// Just start with this for now, change to it if need arises.
// For now, this is easier to read since the upstream is
// xml based, no other real reason.
type Patch struct {
	GroupID    string `json:"groupId"`
	ArtifactID string `json:"artifactId"`
	Version    string `json:"version"`
	Scope      string `json:"scope"`
}

/*
<!-- dependency versions -->
<slf4j.version>1.7.30</slf4j.version>
-    <logback-version>1.2.10</logback-version>
+    <logback-version>1.2.13</logback-version>
*/
// These are just map[string]string and just a blind overwrite.

type PropertyPatch struct {
	Property string `json:"property"`
	Value    string `json:"value"`
}

// PatchProject will update versions for all matched dependencies
// if they are found in Project.Dependencies. If there is no
// match, it will add the dependency to the project.
// Also does a blind overwrite of any properties with propertyPatches.
// TODO(vaikas): Figure out when / if to use DependencyManagement instead.
func PatchProject(project *gopom.Project, patches []Patch, propertyPatches map[string]string) (*gopom.Project, error) {
	if project == nil {
		return nil, fmt.Errorf("project is nil")
	}
	// If there are no straight up version replacements, but
	// for some reason a dependency is missing, gather them here
	// so that we can add them later.
	missingDeps := make(map[Patch]Patch)
	for _, p := range patches {
		fmt.Printf("Have patch: %s.%s:%s\n", p.GroupID, p.ArtifactID, p.Version)
		missingDeps[p] = p
	}

	// If there are any hard coded dependencies that need to be patched, do
	// that here.
	if project.Dependencies != nil {
		for i, dep := range *project.Dependencies {
			fmt.Printf("Checking DEP: %s.%s:%s\n", dep.GroupID, dep.ArtifactID, dep.Version)
			for _, patch := range patches {
				if dep.ArtifactID == patch.ArtifactID &&
					dep.GroupID == patch.GroupID {
					fmt.Printf("Patching %s.%s from %s to %s with scope: %s\n", patch.GroupID, patch.ArtifactID, dep.Version, patch.Version, patch.Scope)
					(*project.Dependencies)[i].Version = patch.Version
					(*project.Dependencies)[i].Scope = patch.Scope

					// Found it, so remove it from the missing deps
					// This is dump, make it better.
					delete(missingDeps, patch)
				}
			}
		}
	}

	if project.Dependencies != nil {
		for _, dep := range *project.Dependencies {
			fmt.Printf("DEP AFTER patching: %s.%s:%s\n", dep.GroupID, dep.ArtifactID, dep.Version)
		}
	}

	if project.DependencyManagement != nil {
		for i, dep := range *project.DependencyManagement.Dependencies {
			fmt.Printf("Checking DM DEP: %s.%s:%s\n", dep.GroupID, dep.ArtifactID, dep.Version)
			for _, patch := range patches {
				if dep.ArtifactID == patch.ArtifactID &&
					dep.GroupID == patch.GroupID {
					fmt.Printf("Patching DM dep %s.%s from %s to %s with scope: %s\n", patch.GroupID, patch.ArtifactID, dep.Version, patch.Version, patch.Scope)
					(*project.DependencyManagement.Dependencies)[i].Version = patch.Version
					(*project.DependencyManagement.Dependencies)[i].Scope = patch.Scope
					// Found it, so remove it from the missing deps
					// This is dump, make it better.
					delete(missingDeps, patch)
				}
			}
		}
	}

	// If there are any missing dependencies, add them in. I guess add them
	// to DependencyManagement?
	if project.DependencyManagement == nil {
		project.DependencyManagement = &gopom.DependencyManagement{}
	}
	for md := range missingDeps {
		md := md
		fmt.Printf("Adding missing dependency: %s.%s:%s\n", md.GroupID, md.ArtifactID, md.Version)

		*project.DependencyManagement.Dependencies = append(*project.DependencyManagement.Dependencies, gopom.Dependency{
			GroupID:    md.GroupID,
			ArtifactID: md.ArtifactID,
			Version:    md.Version,
			Scope:      md.Scope,
		})
	}
	if project.Properties == nil {
		project.Properties = &gopom.Properties{Entries: propertyPatches}
	} else {
		for k, v := range propertyPatches {
			project.Properties.Entries[k] = v
		}
	}
	return project, nil
}
