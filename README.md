# Discord role sync

## Getting started

### Configure and run the application

Get the `config_example.yaml` and `docker-compose.yml` files\
Copie the example config file to `config.yaml`

```console
cp config_example.yaml config.yaml
```

Open the config.yaml file with your favorite text editor and fill it in

```yaml
discord:
  token: ""
```

Then run the application

```console
docker compose up -d
```
