# conso-dashboard

Une commande Go qui récupère la courbe de consommation d'un compteur Linky via [Conso API](https://conso.boris.sh/) et la conserve dans DuckDB.

## Configuration

```sh
cp .env.example .env
```

Renseignez dans `.env` le token `CONSO_API_TOKEN` et le numéro de compteur `CONSO_API_PRM` à 14 chiffres.

## Importer les 30 derniers jours

```sh
set -a
. ./.env
set +a
go run .
```

Pour choisir une période :

```sh
go run . -start 2026-07-01 -end 2026-07-29
```

La commande découpe automatiquement les périodes pour respecter la limite de Conso API. Elle crée `data/conso.duckdb` et alimente la table `consumption_load_curve`. Un nouvel import met à jour les créneaux existants sans créer de doublons.

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
