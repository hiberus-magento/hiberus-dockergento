# hm composer

Execute composer command in container.

## Usage

```bash
hm composer [ARGUMENTS]
```

## Description

The `composer` command is used to launch a composer command on container.

```bash
hm composer [ install | update | ... ]
```

## Arguments
- `create-project` composer subcommand is unavailable. Use `hm create-project` 
- In Mac OS this command syncs host and container during process.

## On macOS

`install`, `update`, `require` and `remove` copy the host's `vendor` into the php container, run Composer there, then stop phpfpm, empty the host's `vendor` and copy the project back out. The project's `.git` is never part of either copy.

Once phpfpm has been stopped for the copy back out, it is started again whatever happens next. When a step after the stop fails, that step's error is what is reported; if phpfpm could not be started again either, the way out says so and names `hm start`.