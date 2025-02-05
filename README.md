# ADCM Installer

### Build
```shell
make build
```

### Usage
Configure project (persistent installation)
```shell
# see `adi init --help` command
adi init adcm-project --adcm --pg -i
# ...
adi apply
```

Stop ADCM
```shell
# see `adi delete --help` command
adi delete
```
