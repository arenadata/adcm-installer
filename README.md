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

### ADCM Secret Storage

ADCM stores its secrets either on filesystem (default) or in a
Vault/OpenBao server. Storage selected during `adi init`:

```shell
# embedded Vault: adds a managed OpenBao service and configures ADCM to use it.
# During `adi apply` OpenBao server started in persistence mode,
# initialized, unsealed and a KV v2 mount point is created for each ADCM
# instance. Root token is passed to ADCM via a vault-token secret
adi init adcm-project --adpg --vault

# interactive mode: select secret storage (FileSystem or Vault) and, for
# Vault, the type (embedded or external)
adi init adcm-project --adpg -i

# external Vault via config file (requires an existing KV v2 mount point)
cat config.yaml
secret-storage: Vault
vault-type: external
adcm-vault-url: https://vault.example.com:8200
adcm-vault-token-file: /path/to/token
adcm-vault-mount-point: adcm
adcm-vault-ca-file: /path/to/ca.crt

adi init adcm-project --adpg --from-config config.yaml
```

### Run init with values from config file

```shell
cat config.yaml
adcm-db-host: pg.example.com
adcm-db-pass: $_ecRet

adi init adcm-project --from-config config.yaml
```

| key                    | value type | default                        | description                              |
|------------------------|------------|--------------------------------|------------------------------------------|
| adcm-count             | uint8      | 1                              | Number of ADCM instances. They share the image, the database and the data volume, and publish consecutive ports starting from adcm-publish-port |
| adcm-worker-count      | uint8      | 1 (celery), unused for local   | Number of ADCM worker (Celery) instances in the installation |
| adcm-db-host           | string     |                                | ADCM database host                       |
| adcm-db-port           | uint16     | 5432                           | ADCM database port                       |
| adcm-db-name           | string     | adcm                           | ADCM database name                       |
| adcm-db-user           | string     | adcm                           | ADCM database user                       |
| adcm-db-pass           | string     | random generated               | ADCM database password                   |
| adcm-db-ssl-mode       | string     | disable                        | Postgres SSL mode                        |
| adcm-db-ssl-ca-file    | string     |                                | ADCM database SSL CA file path           |
| adcm-db-ssl-cert-file  | string     |                                | ADCM database SSL certificate file path  |
| adcm-db-ssl-key-file   | string     |                                | ADCM database SSL private key file path  |
| adcm-ssl-cert-file     | string     |                                | ADCM SSL Certificate file path           |
| adcm-ssl-key-file      | string     |                                | ADCM SSL Private Key file path           |
| adcm-image             | string     | hub.arenadata.io/adcm/adcm     | ADCM image                               |
| adcm-tag               | string     | 2.7.1                          | ADCM image tag                           |
| adcm-publish-port      | uint16     | 8000                           | ADCM publish port                        |
| adcm-publish-ssl-port  | uint16     | 8443                           | ADCM publish SSL port                    |
| adcm-url               | string     | computed                       | ADCM url                                 |
| adcm-volume            | string     | adcm                           | ADCM volume name or path                 |
| secret-storage         | string     | FileSystem (Vault with --vault) | ADCM Secret Storage (FileSystem, Vault) |
| vault-type             | string     | embedded                       | Vault Secret Storage type (embedded, external) |
| job-execution-environment | string  | local (celery with adcm-worker-count) | Where ADCM jobs are executed (local, celery). celery adds ADCM worker services and requires ADCM 3.0.0 or newer |
| adcm-vault-url         | string     |                                | External Vault url                       |
| adcm-vault-token-file  | string     |                                | Path to a file with the external Vault token |
| adcm-vault-mount-point | string     | adcm                           | Vault KV v2 mount point shared by the ADCM instances |
| adcm-vault-ca-file     | string     |                                | External Vault CA file path              |
| adcm-vault-client-cert-file | string |                                | External Vault client certificate file path |
| adcm-vault-client-key-file  | string |                                | External Vault client private key file path |
| adpg-pass              | string     | random generated               | ADPG superuser password                  |
| adpg-image             | string     | hub.arenadata.io/adcm/postgres | ADPG image                               |
| adpg-tag               | string     | v16.3.1                        | ADPG image tag                           |
| adpg-publish-port      | uint16     |                                | ADPG publish port                        |
| consul-image           | string     | hub.arenadata.io/adcm/consul   | Consul image                             |
| consul-tag             | string     | v1.0.0                         | Consul image tag                         |
| consul-publish-port    | uint16     | 8500                           | Consul publish port                      |
| vault-db-host          | string     |                                | Vault database host                      |
| vault-db-port          | uint16     | 5432                           | Vault database port                      |
| vault-db-name          | string     | adcm                           | Vault database name                      |
| vault-db-user          | string     | adcm                           | Vault database user                      |
| vault-db-pass          | string     | random generated               | Vault database password                  |
| vault-db-ssl-mode      | string     | disable                        | Postgres SSL mode                        |
| vault-db-ssl-ca-file   | string     |                                | Vault database SSL CA file path          |
| vault-db-ssl-cert-file | string     |                                | Vault database SSL certificate file path |
| vault-db-ssl-key-file  | string     |                                | Vault database SSL private key file path |
| vault-ssl-cert-file    | string     |                                | Vault SSL Certificate file path          |
| vault-ssl-key-file     | string     |                                | Vault SSL Private Key file path          |
| vault-image            | string     | openbao/openbao                | Vault image                              |
| vault-tag              | string     | 2.2.0                          | Vault image tag                          |
| vault-publish-port     | uint16     | 8200                           | Vault publish port                       |
| vault-mode             | string     | non-ha                         | Vault Deployment mode (non-ha, ha, dev)  |
| vault-ui               | bool       | true                           | Vault enable UI                          |
