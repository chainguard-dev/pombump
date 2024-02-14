# pombump

Programmatically manipulate maven (pom.xml) dependencies.

# Overview

For easier patchability, add ways to selectively bump versions for dependencies.

The idea is just like [gobump](https://github.com/chainguard-dev/gobump) but for
java.

# Usage

The idea is that there are some `patches` that should be applied to the upstream
pom.xml file. You can specify these via `--dependencies` flag, or via
`--patch-file`. You can also update / add Properties using the `--properties`
flag, or via `--properties-file`.

## Specifying Dependencies to be patched

You can specify the patches that should be applied two ways. They are mutually
exclusive, so you can only specify one of them at the time.

### --dependencies flag

You can specify patches via `--dependencies` flag by encoding them
(similarly to gobump) in the following format:

```shell
--dependencies="<groupID@artifactID@version[@scope[@type]]> <groupID...>"
```

So the `groupID`, `artifactID`, and `version` are required fields, and the
`scope`, and `type` are optional fields. If omitted, `scope` defaults to
`import`, and `type` defaults to `jar`.

### --patch-file flag

You can specify a yaml file that contains the patches, which is the preferred
way, because it's less errorprone, and allows for inline comments to keep track
of which patches are for which CVEs. `scope`, and `type` are optional here as
well. If omitted, `scope` defaults to `import`, and `type` defaults to `jar`.

An example yaml file looks like this:
```yaml
patches:
  # CVE-2023-34062
  - groupID: io.projectreactor.netty
    artifactID: reactor-netty-http
    version: 1.0.39
    scope: import
    type: pom
  # CVE-2023-5072
  - groupId: org.json
    artifactId: json
    version: "20231013"
  # CVE-2023-6378
  - groupId: ch.qos.logback
    artifactId: logback-core
    version: "[1.4.12,2.0.0)"
```

## Specifying Properties to be patched

You can specify the properties that should be modified two ways. They are
mutually exclusive, so you can only specify one of them at the time.

### --properties flag

You can specify the properties via `--properties` flag by encoding them in the
(similarly to gobump) in the following format:

```shell
--properties="property@value property@value"
```
### --properties-file flag

You can specify a yaml file that contains the properties that should be
modified. This again is the preferred way for all the same reasons the
`--patch-file` is the preferred way.

An example file looks like so:
```yaml
properties:
  - property: "prop1"
    value: "value1"
  - property: "prop2"
    value: "value2"
```
# Theory of operation

## Patches

Once you have specified the patches, the tool will go through the pom.xml file
and then for each `patch` the following happens:

* If the patch is found in the `dependencies` section, it will be patched
inline.
* If the patch is found in the `dependencyManagement.dependencies` section, it
will be patched inline.
* Otherwise, it will be appended to the `dependencyManagement.dependencies`
section.

## Properties

They are either patched inline (if found), or added to the `properties` section.
