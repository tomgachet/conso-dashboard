# conso-dashboard

Une application Go pour importer et visualiser la consommation électrique d'un compteur Linky.

Elle récupère la courbe de consommation via [Conso API](https://conso.boris.sh/), la conserve localement dans DuckDB et fournit un dashboard web pour explorer les consommations quotidiennes et leur détail intrajournalier.

Le projet reste volontairement simple et autonome :

- un exécutable Go unique pour l'import et le serveur HTTP ;
- la bibliothèque standard `net/http` pour exposer le dashboard et son API ;
- une interface en HTML, CSS et JavaScript natifs, embarquée dans l'exécutable ;
- une base DuckDB locale, embarquée dans l'exécutable et stockée dans un simple fichier, sans installation séparée ;
- aucun framework frontend, service de base de données ou processus supplémentaire.

## Installation

Téléchargez l'archive Linux amd64 depuis la [dernière release](https://github.com/tomgachet/conso-dashboard/releases/latest), puis extrayez-la :

```sh
tar -xzf conso-dashboard-linux-amd64.tar.gz
./conso-dashboard --version
```

Le nom de l'exécutable reste `conso-dashboard`. La version est portée par la release et le nom de l'archive.

## Configuration

Créez un fichier `.env` dans le dossier depuis lequel vous lancerez l'application. Depuis les sources, vous pouvez copier `.env.example`. Renseignez votre token Conso API et le numéro PRM à 14 chiffres de votre compteur :

```dotenv
CONSO_API_TOKEN=votre-token
CONSO_API_PRM=12345678901234
```

Le fichier `.env` est chargé automatiquement au démarrage et reste ignoré par Git. Une variable déjà définie dans le terminal est prioritaire sur la valeur du fichier.

## Importer les 30 derniers jours

```sh
./conso-dashboard
```

Depuis les sources, utilisez `go run .` à la place de `./conso-dashboard`.

Pour choisir une période :

```sh
./conso-dashboard -start 2026-07-01 -end 2026-07-29
```

La date de début est incluse et la date de fin est exclue. La commande découpe automatiquement les périodes pour respecter la limite de Conso API. Elle crée `data/conso.duckdb` et alimente la table `consumption_load_curve`. Un nouvel import met à jour les créneaux existants sans créer de doublons.

## Visualiser les consommations

Après avoir importé des données, lancez le serveur web :

```sh
./conso-dashboard serve
```

Ouvrez ensuite <http://localhost:8080>. Le dashboard affiche :

- la consommation quotidienne sur 7, 30 ou 90 jours ;
- la liste des journées, colorée selon leur niveau de consommation ;
- le détail intrajournalier d'une journée sélectionnée ;
- le numéro du point de consommation associé aux données.

Les données restent stockées localement dans DuckDB : le dashboard les consulte sans contacter Conso API. Pour changer l'adresse d'écoute :

```sh
./conso-dashboard serve -addr :9090
```

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
