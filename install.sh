#!/bin/bash
set +x
set -euo pipefail

SOURCE_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
BIN_DIR=/usr/local/bin
STATE_DIR=/var/lib/conso-dashboard
UNIT_DIR=/etc/systemd/system
server=conso-dashboard.service
timer=conso-dashboard-fetch.timer
daily=conso-dashboard-fetch.service
work_dir=
restore_server=0
restore_timer=0

fail() { echo "Erreur : $*" >&2; exit 1; }
active() {
    local state
    state=$(systemctl show --property=ActiveState --value "$1") || fail "lecture de $1 impossible"
    [[ $state == active || $state == activating || $state == deactivating || $state == reloading ]]
}
cleanup() {
    local result=$?
    trap - EXIT INT TERM HUP
    set +e
    if (( restore_server )); then systemctl start "$server" || result=1; fi
    if (( restore_timer )); then systemctl start "$timer" || result=1; fi
    if [[ -n $work_dir ]]; then rm -rf -- "$work_dir"; fi
    if (( result != 0 )); then
        echo 'Installation interrompue. La configuration et les données sont conservées ; corriger l’erreur puis relancer ./install.sh.' >&2
    fi
    exit "$result"
}
check_platform() {
    [[ -f /etc/os-release ]] || fail 'Debian ou Ubuntu requis'
    . /etc/os-release
    [[ $ID == debian || $ID == ubuntu ]] || fail 'Debian ou Ubuntu requis'
    [[ -d /run/systemd/system ]] || fail 'systemd doit être le gestionnaire de services de cette machine'
    local version
    version=$(systemctl --version)
    version=${version#systemd }
    version=${version%% *}
    version=${version%%$'\n'*}
    [[ $version =~ ^[0-9]+$ ]] && (( version >= 249 )) || fail 'systemd 249 ou supérieur requis'
    case $(uname -m) in
        x86_64) go_arch=amd64 ;;
        aarch64) go_arch=arm64 ;;
        *) fail 'Architecture prise en charge : amd64 ou arm64' ;;
    esac
}
prepare_binary() {
    apt-get update
    apt-get install -y --no-install-recommends ca-certificates curl tzdata util-linux passwd libstdc++6
    if [[ -f $SOURCE_DIR/conso-dashboard ]]; then
        install -m 0755 "$SOURCE_DIR/conso-dashboard" "$work_dir/conso-dashboard"
    else
        [[ -f $SOURCE_DIR/go.mod ]] || fail 'Binaire ou sources Go absents du dossier d’installation'
        apt-get install -y --no-install-recommends build-essential python3
        curl --fail --silent --show-error --location 'https://go.dev/dl/?mode=json&include=all' -o "$work_dir/versions.json"
        # Même série Go que le workflow de release ; dernière révision stable.
        python3 - "$work_dir/versions.json" "$go_arch" > "$work_dir/go-download" <<'PY'
import json, re, sys
with open(sys.argv[1]) as source:
    versions = json.load(source)
for version in versions:
    if version.get('stable') and re.fullmatch(r'go1\.26\.\d+', version['version']):
        for file in version['files']:
            if file['os'] == 'linux' and file['arch'] == sys.argv[2] and file['kind'] == 'archive':
                if not re.fullmatch(r'go1\.26\.\d+\.linux-(amd64|arm64)\.tar\.gz', file['filename']):
                    raise SystemExit('Nom d’archive Go invalide')
                if not re.fullmatch(r'[0-9a-f]{64}', file['sha256']):
                    raise SystemExit('SHA-256 Go invalide')
                print(file['filename'], file['sha256'])
                raise SystemExit(0)
raise SystemExit('Archive Go 1.26 introuvable pour cette architecture')
PY
        local archive checksum
        read -r archive checksum < "$work_dir/go-download"
        curl --fail --silent --show-error --location "https://go.dev/dl/$archive" -o "$work_dir/$archive"
        (cd "$work_dir" && printf '%s  %s\n' "$checksum" "$archive" | sha256sum --check --status)
        tar -xzf "$work_dir/$archive" -C "$work_dir"
        echo 'Compilation de conso-dashboard…'
        (cd "$SOURCE_DIR" && env GOENV=off GOTOOLCHAIN=local CGO_ENABLED=1 \
            GOPATH="$work_dir/gopath" GOCACHE="$work_dir/gocache" \
            "$work_dir/go/bin/go" build -trimpath -o "$work_dir/conso-dashboard" .)
    fi
    "$work_dir/conso-dashboard" --version
}
read_credentials() {
    read -r -s -p 'Token Conso API : ' token < /dev/tty
    printf '\n' > /dev/tty
    read -r -p 'PRM (14 chiffres) : ' prm < /dev/tty
}
configure() {
    if ! id conso-dashboard >/dev/null 2>&1; then
        useradd --system --user-group --home-dir "$STATE_DIR" --shell /usr/sbin/nologin conso-dashboard
    fi
    [[ $(id -u conso-dashboard) != 0 ]] || fail 'Le compte conso-dashboard ne doit pas être root'
    install -d -o conso-dashboard -g conso-dashboard -m 0750 "$STATE_DIR" "$STATE_DIR/data"
    [[ ! -L $STATE_DIR/.env ]] || fail 'Le fichier .env ne doit pas être un lien symbolique'
    if [[ -e $STATE_DIR/.env ]]; then
        echo 'Configuration existante conservée.'
    else
        local token prm
        read_credentials
        [[ -n $token && ! $token =~ [[:space:]] ]] || fail 'Token vide ou contenant des espaces'
        [[ $prm =~ ^[0-9]{14}$ ]] || fail 'Le PRM doit comporter 14 chiffres'
        (umask 077; printf 'CONSO_API_TOKEN=%s\nCONSO_API_PRM=%s\n' "$token" "$prm" > "$work_dir/config.env")
        install -o root -g conso-dashboard -m 0640 "$work_dir/config.env" "$STATE_DIR/.env"
        unset token prm
    fi
}
install_files() {
    # Partage le verrou de la commande d’administration pendant le remplacement.
    exec 9>/run/lock/conso-dashboard-ctl.lock
    flock -n 9 || fail 'Un import manuel est en cours ; réessayer après sa fin'
    # Une unité absente lors de la première installation n’a pas à être arrêtée.
    if [[ $(systemctl show --property=LoadState --value "$timer") == loaded ]]; then
        active "$timer" && restore_timer=1
        systemctl stop "$timer"
    fi
    if [[ $(systemctl show --property=LoadState --value "$server") == loaded ]]; then
        active "$server" && restore_server=1
    fi
    if [[ $(systemctl show --property=LoadState --value "$daily") == loaded ]]; then
        if active "$daily"; then
            restore_server=1
            echo 'Attente de la fin de l’import quotidien…'
            local attempt
            for (( attempt=0; ; attempt++ )); do
                if ! active "$daily"; then break; fi
                if (( attempt >= 450 )); then
                    restore_server=0
                    fail 'L’import quotidien ne s’est pas terminé'
                fi
                sleep 1
            done
        fi
    fi
    if [[ $(systemctl show --property=LoadState --value "$server") == loaded ]]; then
        systemctl stop "$server"
    fi
    install -d -m 0755 "$BIN_DIR" "$UNIT_DIR"
    install -m 0755 "$work_dir/conso-dashboard" "$BIN_DIR/conso-dashboard"
    install -m 0755 "$SOURCE_DIR/deploy/systemd/conso-dashboard-ctl" "$BIN_DIR/conso-dashboard-ctl"
    install -m 0644 "$SOURCE_DIR/deploy/systemd/"*.service "$UNIT_DIR/"
    install -m 0644 "$SOURCE_DIR/deploy/systemd/"*.timer "$UNIT_DIR/"
    systemctl daemon-reload
    flock -u 9
    exec 9>&-
}
start_installation() {
    echo 'Import initial des 30 derniers jours…'
    if ! "$BIN_DIR/conso-dashboard-ctl" fetch; then
        echo 'Consulter : sudo journalctl -u conso-dashboard-manual-fetch.service --since today' >&2
        return 1
    fi
    systemctl enable --now "$server"
    systemctl enable --now "$timer"
    restore_server=0
    restore_timer=0
    systemctl is-active --quiet "$server"
    systemctl is-active --quiet "$timer"
    echo 'Installation terminée. Dashboard : http://127.0.0.1:3457'
    echo 'Import de la veille : chaque jour à 8 h (Europe/Paris).'
    echo 'Historique : sudo conso-dashboard-ctl fetch -start AAAA-MM-JJ -end AAAA-MM-JJ'
    echo 'Logs : sudo journalctl -u conso-dashboard.service -u conso-dashboard-fetch.service -u conso-dashboard-manual-fetch.service -f'
    echo 'Pour un accès distant, utiliser un tunnel SSH ou un reverse proxy.'
}
main() {
    if [[ ${1:-} == --help || ${1:-} == -h ]]; then
        echo 'Utilisation : ./install.sh (Debian/Ubuntu, saisie interactive du token et du PRM)'
        return
    fi
    (( $# == 0 )) || fail 'Argument inconnu ; utiliser ./install.sh --help'
    if (( EUID != 0 )); then exec sudo -- bash "$SOURCE_DIR/install.sh"; fi
    check_platform
    for file in conso-dashboard.service conso-dashboard-fetch.service conso-dashboard-fetch.timer conso-dashboard-ctl; do
        [[ -f $SOURCE_DIR/deploy/systemd/$file ]] || fail "Fichier manquant : $file"
    done
    exec 8>/run/lock/conso-dashboard-install.lock
    flock -n 8 || fail 'Une installation est déjà en cours'
    work_dir=$(mktemp -d /tmp/conso-dashboard-install.XXXXXXXX)
    trap cleanup EXIT
    trap 'exit 130' INT
    trap 'exit 143' TERM
    trap 'exit 129' HUP
    prepare_binary
    configure
    install_files
    start_installation
}
if [[ ${BASH_SOURCE[0]} == "$0" ]]; then main "$@"; fi
