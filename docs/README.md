# `dukkha`

## Syntax

## Common Values

### OS values

- `linux`
- `darwin`
- `windows`
- `freebsd`
- `openbsd`
- `netbsd`

### OS Arch values

- `amd64`
- `armv5`
- `armv6`
- `armv7`
- `arm64`
- `ppc64le`
- `mips64le`
- `riscv64`
- `x86`

## Environment Variables

__NOTE:__ environment variables are also avaiable in template with the same name

### All

- HOST_OS
- HOST_ARCH

- TIME_YEAR
- TIME_MONTH
- TIME_DAY

- GIT_BRANCH
- GIT_COMMIT
- GIT_TAG
- GIT_WORKSPACE_CLEAN
- GIT_ON_DEFAULT_BRANCH

### Tool `go`

- `GO_COMPILER_PLATFORM="$(go version | cut -d\  -f4)"`

### Tool `docker`