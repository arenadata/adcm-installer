# ADCM Installer

### Build
```shell
make build
```

### Usage
Run ADCM in dev mode (each time you run the `adcm up` command, a new database will be created)
```shell
# see `adcm up --help` command
adcm up
```

Stop ADCM
```shell
# see `adcm down --help` command
adcm down
```

Configure project (persistent installation)
```shell
# fast start
adcm up --init
```

```shell
# see `adcm init --help` command
adcm init -i
# Enter PostgreSQL Login (default: adcm):
# Enter PostgreSQL Password:
adcm up
```
