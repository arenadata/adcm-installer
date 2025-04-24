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

### Run init with values from config file
```shell
cat config.yaml
adcm-db-host: pg.example.com
adcm-db-pass: $_ecRet

adi init adcm-project --from-config config.yaml
```

| key                   | value type | default                        | description                             |
| --------------------- | ---------- | ------------------------------ | --------------------------------------- |
| adcm-db-host          | string     |                                | ADCM database host                      |
| adcm-db-port          | uint16     | 5432                           | ADCM database port                      |
| adcm-db-name          | string     | adcm                           | ADCM database name                      |
| adcm-db-user          | string     | adcm                           | ADCM database user                      |
| adcm-db-pass          | string     | random generated               | ADCM database password                  |
| adcm-db-ssl-mode      | string     | disable                        | Postgres SSL mode                       |
| adcm-db-ssl-ca-file   | string     |                                | ADCM database SSL CA file path          |
| adcm-db-ssl-cert-file | string     |                                | ADCM database SSL certificate file path |
| adcm-db-ssl-key-file  | string     |                                | ADCM database SSL private key file path |
| adcm-image            | string     | hub.arenadata.io/adcm/adcm     | ADCM image                              |
| adcm-tag              | string     | 2.6.0                          | ADCM image tag                          |
| adcm-publish-port     | uint16     | 8000                           | ADCM publish port                       |
| adcm-url`*`           | string     | computed                       | ADCM url                                |
| adpg-pass             | string     | random generated               | ADPG superuser password                 |
| adpg-image            | string     | hub.arenadata.io/adcm/postgres | ADPG image                              |
| adpg-tag              | string     | v16.4_arenadata1               | ADPG image tag                          |
| adpg-publish-port     | uint16     |                                | ADPG publish port                       |
| consul-image          | string     | hub.arenadata.io/adcm/consul   | Consul image                            |
| consul-tag            | string     | v0.0.0                         | Consul image tag                        |
| consul-publish-port   | uint16     | 8500                           | Consul publish port                     |
| vault-image           | string     | openbao/openbao                | Vault image                             |
| vault-tag             | string     | 2.2.0                          | Vault image tag                         |
| vault-publish-port    | uint16     |                                | Vault publish port                      |
`*` - not implemented
