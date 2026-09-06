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
- [Démonstration sans compte API](#démonstration-sans-compte-api) : pour découvrir le dashboard avec des données fictives.
- [Tester avec vos données réelles](#télécharger-et-configurer-avec-vos-données-réelles) : pour lancer le dashboard manuellement avec votre compteur.

Ces parcours utilisent l'archive Linux amd64 de la [dernière release](https://github.com/tomgachet/conso-dashboard/releases/latest), avec le binaire déjà compilé. Aucun clonage Git ni installation de Go n'est nécessaire.

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

Ce parcours lance le binaire directement, sans `sudo`, service systemd ni import automatique. Avec vos données réelles, la configuration et les données restent dans le dossier depuis lequel vous lancez les commandes.

### Démonstration sans compte API

Téléchargez et extrayez le binaire, puis lancez la démonstration :

```sh
curl -fLO https://github.com/tomgachet/conso-dashboard/releases/latest/download/conso-dashboard-linux-amd64.tar.gz
tar -xzf conso-dashboard-linux-amd64.tar.gz
./conso-dashboard demo
```

Ouvrez <http://127.0.0.1:3457>. Aucun token, fichier `.env` ou accès à l’API n’est nécessaire. Le dashboard affiche « Données de démonstration » et utilise une base DuckDB en mémoire, indépendante de vos données réelles. Les relevés fictifs couvrent l’année précédente et l’année en cours jusqu’à aujourd’hui inclus, avec des journées complètes de 96 quarts d’heure.

Vous pouvez tester les regroupements par jour, semaine, mois et trimestre, ouvrir le détail d’une journée et parcourir le calendrier, y compris l’année précédente. Les données simulent une consommation plus élevée en hiver et des pointes le matin et le soir. Le PRM `00000000000000` est fictif. La journée d’aujourd’hui est entièrement générée, même si elle n’est pas encore terminée.

Arrêtez avec **Ctrl+C** : les données fictives disparaissent et seront recréées au prochain lancement. Aucun fichier de configuration ou de données réelles n’est lu ni modifié.

Si le port est déjà utilisé, notamment par une installation systemd :

```sh
./conso-dashboard demo -addr 127.0.0.1:3458
```

Ouvrez alors <http://127.0.0.1:3458>. La démo peut fonctionner en parallèle du service existant sur ce port distinct.

Pour tester depuis les sources, avec Go et les prérequis de compilation disponibles, lancez depuis la racine du dépôt :

```sh
go run . demo
# Autre port si nécessaire :
go run . demo -addr 127.0.0.1:3458
```

Le premier lancement depuis les sources peut nécessiter un accès Internet pour télécharger les dépendances Go ; la démo elle-même ne contacte pas Conso API.

La démonstration est réservée à cet essai explicite ; `./install.sh` et les services systemd continuent d’utiliser vos identifiants API et vos données réelles.

### Télécharger et configurer avec vos données réelles

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

- la consommation quotidienne sur la semaine, le mois, le trimestre ou l’année en cours ;
- le regroupement des consommations par jour, semaine, mois ou trimestre ;
- un calendrier annuel des consommations quotidiennes, avec navigation entre les années et couleurs selon le niveau de consommation ;
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

## Limites actuelles

- **Plateforme** : l’archive précompilée est disponible uniquement pour Linux amd64. L’installateur automatique cible Debian / Ubuntu avec systemd 249 ou supérieur.
- **Import quotidien** : le timer lance un import à 8 h, heure de Paris. En cas d’échec API ou de données encore indisponibles, aucune nouvelle tentative automatique n’est programmée dans la journée. Relancez avec `sudo conso-dashboard-ctl fetch yesterday`.
- **Rattrapage après un arrêt** : si la machine était éteinte à l’échéance, le timer importe la veille de son exécution. Après plusieurs jours d’arrêt, les journées plus anciennes doivent être récupérées avec `sudo conso-dashboard-ctl fetch -start AAAA-MM-JJ -end AAAA-MM-JJ` (début inclus, fin exclue).
- **Imports longs** : chaque import est limité à cinq minutes côté application et six minutes côté systemd. Découpez les longues périodes si cette limite est atteinte.
- **Disponibilité pendant l’import** : les imports gérés par systemd ou la commande d’administration arrêtent temporairement le dashboard pour libérer l’accès à DuckDB, puis le relancent.
- **Version depuis les sources** : un binaire compilé par `./install.sh` affiche actuellement `dev`. Les binaires de release affichent le tag de version dans le dashboard et avec `--version`.

## Tests

```sh
go test ./...
```

## Licence

Ce projet est distribué sous licence MIT.
