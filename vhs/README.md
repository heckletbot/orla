# VHS Tapes for Orla

This repository contains [vhs](https://github.com/charmbracelet/vhs) tapes for orla demos. 

Prereqs:

- Install [vhs](https://github.com/charmbracelet/vhs?tab=readme-ov-file#installation) for running
these. 
- The main demo required [glow](https://github.com/charmbracelet/glow).

To create a new tape, run:

```bash
vhs new foo.tape
```

To run a tape, do:

```bash
vhs foo.tape
```

Please make sure that the tapes are saved in the `share/` directory in the repository root.

## Running the demo tape

Add your user to the docker group on the recording machine (`sudo usermod -aG docker $USER`, then log out and back in) so docker runs without sudo.

Start backends and wait for health checks to pass:

```bash
docker compose -f deploy/docker-compose.workflow-demo.yaml up -d
```

Make the runner executable:

```bash
chmod +x vhs/demo_run.sh
```

Do a dry run to measure timing, then adjust `Sleep 40s` in `demo.tape`:

```bash
time ./vhs/demo_run.sh
```

Record (from repo root):

```bash
vhs vhs/demo.tape
```

Output: `share/demo.mp4`