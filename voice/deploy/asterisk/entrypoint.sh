#!/bin/sh
# Renders the Asterisk config templates from the environment and starts Asterisk.
set -eu
: "${SIP_HOST:?set SIP_HOST in .env}"
: "${SIP_USER:?set SIP_USER in .env}"
: "${SIP_PASSWORD:?set SIP_PASSWORD in .env}"
: "${PUBLIC_IP:?set PUBLIC_IP in .env}"
: "${VOICE_HOST:=127.0.0.1}"
export SIP_HOST SIP_USER SIP_PASSWORD PUBLIC_IP VOICE_HOST

render() {
  # envsubst may be missing in slim images: fall back to sed
  if command -v envsubst >/dev/null 2>&1; then
    envsubst '${SIP_HOST} ${SIP_USER} ${SIP_PASSWORD} ${PUBLIC_IP} ${VOICE_HOST}' <"$1" >"$2"
  else
    sed -e "s|\${SIP_HOST}|$SIP_HOST|g" -e "s|\${SIP_USER}|$SIP_USER|g" -e "s|\${SIP_PASSWORD}|$SIP_PASSWORD|g" \
        -e "s|\${PUBLIC_IP}|$PUBLIC_IP|g" -e "s|\${VOICE_HOST}|$VOICE_HOST|g" "$1" >"$2"
  fi
}

render /templates/pjsip.conf.template /etc/asterisk/pjsip.conf
# extensions.conf keeps Asterisk ${...} variables: only VOICE_HOST is substituted
sed -e "s|\${VOICE_HOST}|$VOICE_HOST|g" /templates/extensions.conf.template >/etc/asterisk/extensions.conf
cat >/etc/asterisk/modules.conf <<'EOF'
[modules]
autoload=yes
noload => chan_sip.so
load => res_audiosocket.so
load => app_audiosocket.so
load => func_curl.so
load => func_shell.so
EOF
exec asterisk -f -vvv
