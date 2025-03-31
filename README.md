# ADCM Installer

### Build
```shell
make build
# or
docker run --rm -it -v $HOME/go/pkg/mod:/go/pkg/mod -v `pwd`:/app golang:1.24 /bin/bash -c 'cd /app && make build'
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
