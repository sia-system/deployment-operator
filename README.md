# artifactor
Utility running in init container of POD for copy build artifact from git release to main container

## configuration

> config file must be in: /etc/artifactor/config.yaml

see example of config.yaml in source

## environment variables

- PROVIDER
- GROUP
- PROJECT
- VOLUME
- APP_SERVER_MODE
- ASSETS_SRC
- ASSETS_DST