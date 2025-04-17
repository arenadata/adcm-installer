# ADCM Installer

### Build
build binary file for current OS/Arch. **This is the only way to build under MacOS**
```shell
make build
```

build binary file for linux amd64
```shell
make linux
# or
make in-docker
```

### Usage
Configure project (persistent installation)
```shell
# see `adi init --help` command
adi init adcm-project --adpg -i
# ...
adi apply
```

Stop ADCM
```shell
# see `adi delete --help` command
adi delete
```
