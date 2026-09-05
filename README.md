# conso-dashboard

Une application Go pour importer et visualiser la consommation électrique d'un compteur Linky.

Elle récupère la courbe de consommation via [Conso API](https://conso.boris.sh/), la conserve localement dans DuckDB et fournit un dashboard web pour explorer les consommations quotidiennes et leur détail intrajournalier.

Le projet reste volontairement simple et autonome :

- un exécutable Go unique pour l'import et le serveur HTTP ;
- la bibliothèque standard `net/http` pour exposer le dashboard et son API ;
- une interface en HTML, CSS et JavaScript natifs, embarquée dans l'exécutable ;
- une base DuckDB locale, embarquée dans l'exécutable et stockée dans un simple fichier, sans installation séparée ;
- aucun framework frontend, service de base de données ou processus supplémentaire.

## Choisir un mode d'utilisation

- [Installation automatique avec systemd](#installation-automatique-avec-systemd) : pour faire tourner le dashboard en continu, avec import quotidien automatique.
- [Tester sans installation](#tester-sans-installation) : pour lancer le dashboard manuellement dans un dossier local.

Les deux parcours utilisent l'archive Linux amd64 de la [dernière release](https://github.com/tomgachet/conso-dashboard/releases/latest), avec le binaire déjà compilé. Aucun clonage Git ni installation de Go n'est nécessaire.

## Installation automatique avec systemd

Consultez le [guide complet d’installation systemd](deploy/systemd/README.md) pour les détails de configuration et d’administration.

Sur **Debian / Ubuntu avec systemd 249 ou supérieur**, téléchargez l'archive et lancez l'installateur :

```sh
curl -fLO https://github.com/tomgachet/conso-dashboard/releases/latest/download/conso-dashboard-linux-amd64.tar.gz
tar -xzf conso-dashboard-linux-amd64.tar.gz
./install.sh
```

L'installateur demande les droits `sudo`, votre **token Conso API** (saisie masquée) et le **PRM à 14 chiffres** de votre compteur. Il s'occupe ensuite de :

- créer le compte système et installer les fichiers ;
- enregistrer la configuration dans `/var/lib/conso-dashboard/.env` ;
- importer les 30 derniers jours dans `/var/lib/conso-dashboard/data/conso.duckdb` ;
- démarrer le dashboard et l'activer au démarrage de la machine ;
- programmer l'import de la veille chaque jour à **8 h, heure de Paris**.

Après réussite, le dashboard est accessible sur <http://127.0.0.1:3457> depuis la machine installée. Pour un serveur distant, utilisez un tunnel SSH ou un reverse proxy.

### Importer davantage de données

Une fois le service installé, utilisez la commande d'administration depuis n'importe quel dossier :

```sh
sudo conso-dashboard-ctl fetch -start 2026-01-01 -end 2026-09-01
```

La date de début est incluse et la date de fin est exclue. Cette commande gère l'arrêt du dashboard pendant l'import et sa remise en service.

### Consulter les logs

```sh
# Dashboard
sudo journalctl -u conso-dashboard.service -f

# Imports quotidiens
sudo journalctl -u conso-dashboard-fetch.service --since today

# Imports manuels
sudo journalctl -u conso-dashboard-manual-fetch.service --since today
```

Le [guide systemd](deploy/systemd/README.md) détaille l'accès réseau, la rétention des logs, les sauvegardes et les mises à jour.

## Tester sans installation

Ce parcours lance le binaire directement, sans `sudo`, service systemd ni import automatique. La configuration et les données restent dans le dossier depuis lequel vous lancez les commandes.

### Télécharger et configurer

```sh
curl -fLO https://github.com/tomgachet/conso-dashboard/releases/latest/download/conso-dashboard-linux-amd64.tar.gz
tar -xzf conso-dashboard-linux-amd64.tar.gz
cp .env.example .env
nano .env
```

Renseignez votre token Conso API et le PRM à 14 chiffres de votre compteur dans `.env` :

```dotenv
CONSO_API_TOKEN=votre-token
CONSO_API_PRM=12345678901234
```

La commande `fetch` charge automatiquement ce fichier. Une variable déjà définie dans le terminal est prioritaire sur la valeur du fichier.

### Importer et lancer le dashboard

Importez les 30 derniers jours, puis démarrez le serveur :

```sh
./conso-dashboard fetch
./conso-dashboard serve -addr 127.0.0.1:3457
```

Ouvrez <http://localhost:3457>. La base est créée dans `data/conso.duckdb` et les logs s'affichent dans le terminal. Utilisez **Ctrl+C** pour arrêter le serveur.

### Refaire un import

Arrêtez d'abord le serveur avec `Ctrl+C` pour libérer DuckDB, puis choisissez une commande :

```sh
# Les 30 derniers jours
./conso-dashboard fetch

# La veille, calculée dans le fuseau horaire local
./conso-dashboard fetch yesterday

# Une période précise : début inclus, fin exclue
./conso-dashboard fetch -start 2026-07-01 -end 2026-07-29
```

Un nouvel import met à jour les créneaux existants sans créer de doublons. Relancez ensuite le dashboard :

```sh
./conso-dashboard serve -addr 127.0.0.1:3457
```

Pour changer le port, remplacez `127.0.0.1:3457` par `127.0.0.1:9090`.

## Fonctionnalités du dashboard

Le dashboard affiche :

- la consommation quotidienne sur la semaine, le mois ou l'année en cours ;
- la liste des journées, colorée selon leur niveau de consommation ;
- le détail intrajournalier d'une journée sélectionnée ;
- le numéro du point de consommation associé aux données.

Les données restent stockées localement dans DuckDB : le dashboard les consulte sans contacter Conso API.

## Données stockées

Chaque ligne contient notamment l'horodatage `reading_at`, la puissance moyenne `value_w` en watts et la durée `interval_length`. Le compteur fournit actuellement des intervalles de 15 minutes (`PT15M`).

La consommation quotidienne en kWh se calcule directement depuis ces mesures :

```sql
SELECT
    CAST(reading_at - INTERVAL 1 MICROSECOND AS DATE) AS day,
    SUM(value_w * 0.25) / 1000 AS consumption_kwh
FROM consumption_load_curve
WHERE interval_length = 'PT15M'
GROUP BY day
ORDER BY day;
```

## Tests

```sh
go test ./...
```

## Licence

Ce projet est distribué sous licence MIT.
