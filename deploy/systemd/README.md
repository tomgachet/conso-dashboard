# Installation avec systemd

Le dashboard démarre au boot et redémarre en cas d'échec. Un timer importe les données de la veille chaque jour à **8 h, heure de Paris**, avec `fetch yesterday`. Prérequis : Linux avec **systemd 249 ou supérieur** (pour `OnSuccess=`), et l'archive de release extraite dans le dossier courant. Depuis les sources, placer le binaire compilé dans ce dossier.

## Installation automatique sur Debian / Ubuntu

Depuis le dossier de l'archive extraite ou des sources :

```sh
./install.sh
```

Le script demande les droits sudo, vérifie systemd (249 minimum), installe les prérequis avec APT et utilise le binaire fourni. Depuis les sources, il télécharge la dernière révision stable Go 1.26 pour amd64 ou arm64 depuis `go.dev`, vérifie son SHA-256 puis compile dans un dossier temporaire. L'installation Go de la machine est conservée. Un accès Internet est nécessaire pour APT et, depuis les sources, pour Go et ses modules.

Lors de la première installation, il demande le token (saisie masquée) et le PRM, crée le compte dédié et un `.env` protégé, installe les unités et la commande d'administration, puis importe les 30 derniers jours. Après réussite, il active le dashboard et le timer quotidien à 8 h. Le dashboard écoute sur `127.0.0.1:3457` ; pour une machine distante, utiliser un tunnel SSH ou un reverse proxy.

Pour installer directement depuis la branche de développement actuelle (Git et une clé SSH GitHub doivent être disponibles) :

```sh
git clone --branch feat/systemd git@github.com:tomgachet/conso-dashboard.git
cd conso-dashboard
./install.sh
```

Une nouvelle exécution conserve le `.env` et les données, remplace les fichiers livrés et refait l'import initial sans doublons. Elle attend un import quotidien en cours et refuse de remplacer les fichiers pendant un import manuel. Les services précédemment actifs sont relancés en cas d'échec ; lors d'une première installation, un échec d'import empêche l'activation automatique. Corriger au besoin la configuration avec `sudoedit /var/lib/conso-dashboard/.env` puis relancer le script. Les personnalisations systemd doivent être placées dans des drop-ins avec `systemctl edit`.

Les logs sont capturés par journald avec la politique de rétention de la machine ; l'installateur ne modifie pas la rétention globale. Pour reprendre un historique plus long après installation :

```sh
sudo conso-dashboard-ctl fetch -start 2026-01-01 -end 2026-09-01
```

## Installation manuelle

Créer un compte dédié, installer le binaire et renseigner le token et le PRM :

```sh
sudo useradd --system --user-group --home-dir /var/lib/conso-dashboard --shell /usr/sbin/nologin conso-dashboard
sudo install -m 0755 conso-dashboard /usr/local/bin/conso-dashboard
sudo install -d -o conso-dashboard -g conso-dashboard -m 0750 /var/lib/conso-dashboard/data
sudo install -o root -g conso-dashboard -m 0640 .env.example /var/lib/conso-dashboard/.env
sudoedit /var/lib/conso-dashboard/.env
sudo install -m 0644 deploy/systemd/conso-dashboard.service /etc/systemd/system/conso-dashboard.service
sudo install -m 0644 deploy/systemd/conso-dashboard-fetch.service /etc/systemd/system/conso-dashboard-fetch.service
sudo install -m 0644 deploy/systemd/conso-dashboard-fetch.timer /etc/systemd/system/conso-dashboard-fetch.timer
sudo install -m 0755 deploy/systemd/conso-dashboard-ctl /usr/local/bin/conso-dashboard-ctl
sudo systemctl daemon-reload
```

Pour une réinstallation, conserver le compte et le `.env` existants. Le serveur HTTP ne charge pas `.env` ; ce fichier est utilisé par l'import.

Importer les données avant de démarrer le service :

```sh
sudo conso-dashboard-ctl fetch
```

Pour remplir une période historique dès l’installation, utiliser par exemple `sudo conso-dashboard-ctl fetch -start 2026-01-01 -end 2026-09-01` (début inclus, fin exclue). Les mêmes commandes fonctionnent après activation du dashboard.

Après un import réussi :

```sh
sudo systemctl enable --now conso-dashboard.service
sudo systemctl enable --now conso-dashboard-fetch.timer
sudo systemctl list-timers conso-dashboard-fetch.timer
sudo systemctl status conso-dashboard.service
curl --fail http://127.0.0.1:3457/api/info
```

Les données sont stockées dans `/var/lib/conso-dashboard/data/conso.duckdb`. Pour reprendre une base existante, arrêter ses utilisateurs, copier le dossier `data` complet et donner sa propriété au compte `conso-dashboard` avant de démarrer.

L'écoute est limitée à `127.0.0.1:3457`, pour un accès local ou via un reverse proxy. Pour écouter sur le réseau, utiliser `sudo systemctl edit conso-dashboard.service` :

```ini
[Service]
ExecStart=
ExecStart=/usr/local/bin/conso-dashboard serve -addr :3457
```

Puis lancer `sudo systemctl restart conso-dashboard.service`. Le dashboard n'intègre pas d'authentification ; adapter l'accès réseau aux données personnelles affichées.

## Import quotidien

Le timer lance `fetch yesterday` à 8 h dans le fuseau `Europe/Paris`, y compris après les changements d'heure. `TZ=Europe/Paris` détermine aussi la veille côté application.

Systemd arrête le dashboard avant l'import pour libérer DuckDB (`Conflicts=` et `Before=`), puis le redémarre après réussite ou échec (`OnSuccess=` et `OnFailure=`). Le dashboard est indisponible pendant l'import. Un import bloqué est interrompu après six minutes. L'import s'exécute sous le compte dédié et conserve son statut d'échec même si le dashboard redémarre.

`Persistent=true` déclenche un appel de rattrapage si la machine était arrêtée à l'échéance. Cet appel importe uniquement la veille de son exécution, sans reprendre les journées plus anciennes. Si les données ne sont pas encore disponibles à 8 h, consulter les logs et relancer l'import dans la journée ; aucune nouvelle tentative automatique n'est programmée le même jour.

Pour déclencher immédiatement le même import et consulter son résultat :

```sh
sudo systemctl start conso-dashboard-fetch.service
sudo journalctl -u conso-dashboard-fetch.service -n 50 --no-pager
sudo systemctl status conso-dashboard.service
```

Ne pas lancer directement `fetch` pendant que le serveur tourne. Démarrer manuellement le serveur pendant l'import annule celui-ci, les deux services étant exclusifs.

## Import manuel après installation

Depuis n'importe quel dossier :

```sh
# Les 30 derniers jours (remplissage initial ou rattrapage)
sudo conso-dashboard-ctl fetch

# La veille
sudo conso-dashboard-ctl fetch yesterday

# Une période précise : début inclus, fin exclue
sudo conso-dashboard-ctl fetch -start 2026-01-01 -end 2026-09-01
```

La commande suspend le timer, attend la fin d'un import quotidien éventuel, arrête le dashboard et lance l'import sous le compte dédié. Elle rétablit ensuite les services qui étaient actifs, même si l'import échoue ou si la commande reçoit Ctrl+C. Lors du remplissage initial, le dashboard et le timer restent arrêtés jusqu'à leur activation explicite. Un second import manuel simultané est refusé.

Les logs sont accessibles avec `sudo journalctl -u conso-dashboard-manual-fetch.service --since today`. Le code de retour est non nul en cas d'échec. Comme l'import CLI actuel, chaque appel est limité à cinq minutes côté application (six minutes côté systemd) ; découper un historique très long en plusieurs appels si cette limite est atteinte. Les imports peuvent être relancés sans doublons.

## Maintenance et sauvegarde

Pour une sauvegarde ou une mise à jour, arrêter le timer, attendre la fin de l'import éventuel, puis arrêter le dashboard :

```sh
sudo systemctl stop conso-dashboard-fetch.timer
systemctl is-active conso-dashboard-fetch.service
```

Attendre que l'import ne soit plus `activating`, `active` ou `deactivating`, et que le dashboard ait redémarré, avant de continuer :

```sh
sudo systemctl stop conso-dashboard.service
# Effectuer ici la sauvegarde du dossier data complet
sudo systemctl start conso-dashboard.service
sudo systemctl start conso-dashboard-fetch.timer
```

Copier le dossier `data` complet, service arrêté, puis relancer le dashboard et le timer. Arrêter uniquement le dashboard ne suffit pas pour une maintenance : le prochain import programmé le redémarrerait.

## Logs

Chaque `fetch` journalise la période demandée, puis un bilan, par exemple :

```text
fetch: statut=succès récupérés=96 insérés=24 déjà_présents_mis_à_jour=72 non_validés=0 base=data/conso.duckdb
```

- `récupérés` : nombre de relevés contenus dans les réponses API reçues avec succès ;
- `insérés` : nouvelles lignes validées dans DuckDB ;
- `déjà_présents_mis_à_jour` : relevés dont la clé (PRM, horodatage) existait déjà, mis à jour pour conserver les corrections de l'API ;
- `non_validés` : relevés récupérés mais non validés en base en cas d'échec.

Les compteurs portent sur les relevés traités : un doublon dans une même réponse compte comme une insertion puis une mise à jour. Un relevé identique déjà présent compte aussi comme mis à jour (son `fetched_at` est rafraîchi). Les compteurs d'écriture ne sont ajoutés qu'après validation de chaque transaction ; si un lot échoue, les lots précédents restent validés et apparaissent dans le bilan d'échec. Les erreurs d'arguments sont signalées avant tout appel API, sans bilan d'import.

Les messages de démarrage et les erreurs du serveur sont envoyés à journald. Les requêtes HTTP réussies ne sont pas journalisées. Les imports lancés par systemd disposent de leur propre journal :

```sh
sudo journalctl -u conso-dashboard-fetch.service --since today --no-pager
```

Les imports lancés directement dans le terminal y affichent leur résultat.

```sh
# Suivre le serveur
sudo journalctl -u conso-dashboard.service -f

# Consulter les dernières 24 heures
sudo journalctl -u conso-dashboard.service --since '24 hours ago' --no-pager

# Exporter les logs du jour
sudo journalctl -u conso-dashboard.service --since today --no-pager > conso-dashboard.log
```

Journald assure la rotation sans logrotate. Pour conserver les logs après redémarrage et limiter leur volume, créer le dossier `/etc/systemd/journald.conf.d` si nécessaire et y ajouter `retention.conf` :

```ini
[Journal]
Storage=persistent
SystemMaxUse=200M
MaxRetentionSec=30day
```

Ces réglages concernent **l'ensemble du journal système**, pas uniquement conso-dashboard. Ils limitent la rétention sans garantir 30 jours si la limite de volume est atteinte. Appliquer avec :

```sh
sudo systemctl restart systemd-journald.service
sudo journalctl --flush
sudo journalctl --disk-usage
```

Références : [systemd.exec](https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html), [journald.conf](https://www.freedesktop.org/software/systemd/man/latest/journald.conf.html).

## Mise à jour

Suivre la procédure de maintenance ci-dessus pour arrêter le timer et attendre la fin de tout import, puis sauvegarder le dossier `data` service arrêté. Après extraction du nouveau binaire dans le dossier courant :

```sh
sudo systemctl stop conso-dashboard.service
sudo install -m 0755 conso-dashboard /usr/local/bin/conso-dashboard
sudo install -m 0755 deploy/systemd/conso-dashboard-ctl /usr/local/bin/conso-dashboard-ctl
sudo systemctl start conso-dashboard.service
sudo systemctl start conso-dashboard-fetch.timer
/usr/local/bin/conso-dashboard --version
sudo systemctl status conso-dashboard.service
```

Le binaire est séparé de la configuration et des données.
