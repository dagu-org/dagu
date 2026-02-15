#!/usr/bin/env /bin/sh

# Check if both DOCKER_GID is not -1. This indicates the desire for a docker group
if [ "$DOCKER_GID" != "-1" ]; then
  if ! getent group docker >/dev/null; then
    echo "Creating docker group with GID ${DOCKER_GID}"
    addgroup -g ${DOCKER_GID} docker
    usermod -a -G docker boltbase
  fi 

  echo "Changing docker group GID to ${DOCKER_GID}"
  groupmod -o -g "$DOCKER_GID" docker
fi

CURRENT_UID=$(id -u boltbase 2>/dev/null || echo -1)
CURRENT_GID=$(getent group boltbase | cut -d: -f3 2>/dev/null || echo -1)

if [ "$CURRENT_UID" != "$PUID" ] || [ "$CURRENT_GID" != "$PGID" ]; then
    groupmod -o -g "$PGID" boltbase
    usermod -o -u "$PUID" boltbase
fi

mkdir -p ${BOLTBASE_HOME:-/var/lib/boltbase}

# If BOLTBASE_HOME is not set, try to guess if the legacy /home directory is being
# used. If so set the HOME to /home/boltbase. Otherwise force the /var/lib/boltbase directory
# as BOLTBASE_HOME
if [ -z "$BOLTBASE_HOME" ]; then
  # For ease of use set BOLTBASE_HOME to /var/lib/boltbase so all data is located in a
  # single directory following FHS conventions
  export BOLTBASE_HOME=/var/lib/boltbase
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

# Run the command as the boltbase user and optionally the docker group
exec sudo -E -n -u "#${PUID}" -g "#${RUN_GID}" -- "$@"
