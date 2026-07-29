# conso-dashboard

Une commande Go qui récupère la courbe de consommation d'un compteur Linky via [Conso API](https://conso.boris.sh/) et la conserve dans DuckDB.

## Installation

Téléchargez l'archive Linux amd64 depuis la [dernière release](https://github.com/tomgachet/conso-dashboard/releases/latest), puis extrayez-la :

```sh
tar -xzf conso-dashboard-linux-amd64.tar.gz
./conso-dashboard --version
```

Le nom de l'exécutable reste `conso-dashboard`. La version est portée par la release et le nom de l'archive.

## Configuration

Créez un fichier `.env` dans le dossier depuis lequel vous lancerez la commande. Depuis les sources, vous pouvez copier `.env.example`. Renseignez votre token Conso API et le numéro PRM à 14 chiffres de votre compteur :

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
