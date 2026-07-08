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

You can set the path via the `--config` CLI argument.

### Structure

- **dry_run**: (_boolean_) commands are not executed when true (_defaults to false_)
- **log_level**: (_string_) a logrus log level (_defaults to info_)
- **max_threads**: (_integer_) number of app directories to process at one time (_defaults to 3_)
- **command_timeout**: (_string_) a Go duration (e.g. `10m`, `90s`) that each command is allowed to run before it is killed and treated as a failure (_defaults to 15m_)
- **refresh_commands**: (_[][]string_) commands and arguments to run in each directory (_defaults to `docker compose pull && docker compose up -d --remove-orphans`_)
- **after_commands**: (_[][]string_) commands and arguments to run after apps are processed
- **apps**: (_[]object_) configuration for each app:
  - **name**: (_string_) a display name for logs
  - **path**: (_string_) a working directory (for example: where docker-compose.yaml is located)
  - **refresh_commands**: (_[][]string_) commands and arguments to run for this specific app (overriding the global command).
  - **after_commands**: (_[][]string_) commands and arguments to run after this specific app is processed
  - **skip**: (_boolean_) skip this app entirely when true (_defaults to false_)

#### Commands and arguments

These accept the following variables which are replaced before executing the commands:

- `{{app.name}}`: the value of `app.name`
- `{{app.path}}`: the value of `app.path`
- `{{HOME}}`: the value of `$HOME`
