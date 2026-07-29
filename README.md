# conso-dashboard

Première brique d'un tableau de bord personnel : cette commande Go récupère la consommation quotidienne d'un compteur Linky via [Conso API](https://conso.boris.sh/) et la conserve dans un fichier DuckDB.

## Configuration

Obtenez un token sur Conso API, puis créez votre fichier local :

```sh
cp .env.example .env
```

Renseignez `CONSO_API_TOKEN` et le numéro `CONSO_API_PRM` à 14 chiffres. Le fichier `.env` est ignoré par Git.

## Importer les 30 derniers jours

```sh
set -a
. ./.env
set +a
go run .
```

La base est créée dans `data/conso.duckdb`. Pour choisir une période :

```sh
go run . -start 2026-07-01 -end 2026-07-29
```

La date de début est incluse et la date de fin est exclue, conformément à Conso API. Une nouvelle exécution met à jour les journées déjà présentes au lieu de créer des doublons.

## Schéma initial

La table `daily_consumption` contient :

- `prm` : identifiant du compteur ;
- `reading_date` : date du relevé ;
- `value_wh` : consommation quotidienne en Wh ;
- `quality` : qualité fournie par Enedis ;
- `fetched_at` : date de récupération.

La clé primaire est `(prm, reading_date)`.

## Tests

```sh
go test ./...
```
