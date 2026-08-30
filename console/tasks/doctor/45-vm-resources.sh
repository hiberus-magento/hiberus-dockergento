#!/usr/bin/env bash
# Do the containers have enough memory, and enough of the machine's?
source "$HELPERS_DIR"/doctor.sh

#
# On macOS the containers do not run on the laptop, they run in a virtual machine with whatever
# memory somebody gave it once. A machine with 48 GB whose Docker VM has 6 fits about six
# environments, and until this check existed nothing said so — the symptom was environments that
# would not start, on a laptop with plenty of memory free.
#
# On Linux the two numbers are the same and this always passes, which is the correct answer there.
#
info=$(docker info --format '{{.MemTotal}}|{{.NCPU}}|{{.Name}}' 2>/dev/null)

if [ -z "$info" ]; then
    doctor_warning "Could not read how much memory Docker has"
    return 0 2>/dev/null || exit 0
fi

IFS='|' read -r vm_bytes cpus runtime <<< "$info"

host_bytes=$(hm_host_memory_bytes)
# Rounded rather than truncated: 6.2 GB reported as "5" is the kind of small lie that makes
# somebody check the number somewhere else and stop trusting the rest
vm_gb=$(( (vm_bytes + 536870912) / 1073741824 ))
host_gb=$(( (host_bytes + 536870912) / 1073741824 ))

# Measured: an environment on the agent profile is about 550 MB, and a gigabyte is left for
# everything else that runs in there
environments=$(( (vm_bytes / 1024 / 1024 - 1024) / 550 ))
[ "$environments" -lt 0 ] && environments=0

case "$runtime" in
    colima)         action="colima stop && colima start --memory 16 --cpu 4" ;;
    docker-desktop) action="Docker Desktop → Settings → Resources → Memory" ;;
    *)              action="" ;;
esac

case "$(hm_vm_memory_verdict "$vm_bytes" "$host_bytes")" in
    small)
        doctor_error "Docker has ${vm_gb} GB and ${cpus} CPU: not enough for one full stack" "$action"
        ;;
    cramped)
        doctor_warning "Docker has ${vm_gb} GB of this machine's ${host_gb} GB, so about ${environments} environments fit at once" "$action"
        ;;
    unknown)
        doctor_warning "Could not read how much memory Docker has"
        ;;
    *)
        doctor_ok "Docker has ${vm_gb} GB and ${cpus} CPU: about ${environments} environments fit at once"
        ;;
esac
