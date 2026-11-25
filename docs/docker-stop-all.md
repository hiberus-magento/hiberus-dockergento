# hm docker-stop-all

Stop all pulled up containers. 

## Usage

```bash
hm docker-stop-all
```

When the Docker daemon cannot be reached, nothing is stopped and the command fails with exit code
3 (`docker_failed`) and the daemon's message, instead of answering "No containers running".
