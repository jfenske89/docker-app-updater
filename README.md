# docker-app-updater

This is a simple utility to update Docker compose apps.

More advanced use cases could customize the behavior of this utility, so that it runs a series of custom commands in multiple directories.

## Configuration file

See [./config/config.example.json](./config/config.example.json). Copy that to `./config/config.json` and modify as needed.

### Path

**Default paths**:
- ./config/config.json
- ~/.config/update-docker-apps/config.json
- /etc/update-docker-apps/config.json

You can set the path via the `CONFIG_FILE` environment variable.

### Structure

- **dry_run**: (*boolean*) commands are not executed when true (*defaults to false*)
- **log_level**: (*string*) a logrus log level (*defaults to info*)
- **max_threads**: (*integer*) number of app directories to process at one time (*defaults to 3*)
- **refresh_commands**: (*[][]string*) commands and arguments to run in each directory (*defaults to `docker compose pull && docker compose up -d --remove-orphans`*)
- **after_commands**: (*[][]string*) commands and arguments to run after apps are processed
- **apps**: (*[]object*) configuration for each app:
  - **name**: (*string*) a display name for logs
  - **path**: (*string*) a working directory (for example: where docker-compose.yaml is located)
  - **after_commands**: (*[][]string*) commands and arguments to run after this specific app is processed

#### Commands and arguments

These accept the following variables which are replaced before executing the commands:

- `{{app.name}}`: the value of `app.name`
- `{{app.path}}`: the value of `app.path`
- `{{HOME}}`: the value of `$HOME`
