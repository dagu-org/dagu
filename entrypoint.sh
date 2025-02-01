#!/usr/bin/env /bin/sh

echo "Starting entrypoint.sh"

echo "
PUID=${PUID}
PGID=${PGID}
DOCKER_GID=${DOCKER_GID}
TZ=${DAGU_TZ}
"

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

groupmod -o -g "$PGID" dagu
usermod -o -u "$PUID" dagu

mkdir -p /config

chown $PUID:$PGID -R /config

# If DAGU_HOME is not set, try to guess if the legacy /home directory is being
# used. If so set the HOME to /home/dagu. Otherwise force the /config directory
# as DAGU_HOME
if [ -z "$DAGU_HOME" ]; then
  if [ -d /home/dagu/.config/dagu ]; then
    echo "WARNING: Using legacy /home/dagu directory. Please consider moving to /config"
    usermod -d /home/dagu dagu
    chown $PUID:$PGID -R /home/dagu
  else
    # For ease of use set DAGU_HOME to /config so all data is located in a
    # single directory
    export DAGU_HOME=/config
  fi
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
