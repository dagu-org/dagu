#!/usr/bin/env /bin/sh

# Check if both DOCKER_GID is not -1. This indicates the desire for a docker group
if [ "$DOCKER_GID" != "-1" ]; then
  if ! getent group docker >/dev/null; then
    echo "Creating docker group with GID ${DOCKER_GID}"
    addgroup -g ${DOCKER_GID} docker
    usermod -a -G docker dagu
  fi 

  echo "Changing docker group GID to ${DOCKER_GID}"
  groupmod -o -g "$DOCKER_GID" docker
fi

CURRENT_UID=$(id -u dagu 2>/dev/null || echo -1)
CURRENT_GID=$(getent group dagu | cut -d: -f3 2>/dev/null || echo -1)

if [ "$CURRENT_UID" != "$PUID" ] || [ "$CURRENT_GID" != "$PGID" ]; then
    groupmod -o -g "$PGID" dagu
    usermod -o -u "$PUID" dagu
fi

mkdir -p /dagu

# If DAGU_HOME is not set, try to guess if the legacy /home directory is being
# used. If so set the HOME to /home/dagu. Otherwise force the /dagu directory
# as DAGU_HOME
if [ -z "$DAGU_HOME" ]; then
  # For ease of use set DAGU_HOME to /dagu so all data is located in a
  # single directory
  export DAGU_HOME=/dagu
fi

# Run all scripts in /etc/custom-init.d. It assumes that all scripts are
# executable
if [ -d /etc/custom-init.d ]; then
  for f in /etc/custom-init.d/*; do
    if [ -x "$f" ]; then
      echo "Running $f"
      $f
    fi
  done
fi

# If DOCKER_GID is not -1 set RUN_GID to DOCKER_GID otherwise set to PGID
if [ "$DOCKER_GID" != "-1" ]; then
  RUN_GID=$DOCKER_GID
else
  RUN_GID=$PGID
fi

# Run the command as the dagu user and optionally the docker group
exec sudo -E -n -u "#${PUID}" -g "#${RUN_GID}" -- "$@"
