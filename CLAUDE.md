# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## État du projet

V1 implémentée selon la spécification (module `elecsubscribesimul`, Go 1.25), avec tests : `go build`, `go vet` et `go test ./...` passent (le paquet `internal/export` n'a pas de tests). Pas de dépôt git initialisé.

`testdata/` contient désormais des données réelles : ~40 relevés Enedis hebdomadaires (`22184080962179_Export_energie_Consommation_JJMMAAAA-JJMMAAAA.xlsx`, du 26/01/2026 au 24/10/2026, un seul PRM), `tempo.csv` (couleurs Tempo du 2025-12-29 au 2026-10-31) et `abonnements.yaml` (offres « Zen » 12 kVA : Fixe Base, Fixe HP/HC, Fixe Online, Online HP/HC, Tempo ; prix non contractuels). Ils servent de jeu d'essai de bout en bout (`--courbe testdata --couleurs testdata/tempo.csv`) ; les tests unitaires, eux, génèrent toujours des fichiers synthétiques avec excelize. Ne pas modifier ces fichiers de données.

Les documents sont rédigés en français ; garder le français pour la documentation et les messages utilisateur du programme (erreurs, avertissements, libellés Excel, options CLI, clés YAML). Le code (identifiants, commentaires) est en anglais : voir `Docs/design-rules.md` §6.

## Documents de référence (à lire avant de coder)

- `Docs/SPEC_simulateur_abonnement_electricite.md` : spécification fonctionnelle et technique complète, avec exigences numérotées (`REQ-IN-*`, `REQ-CALC-*`, `REQ-TIME-*`, `REQ-OUT-*`). Citer ces identifiants dans les tests et les commentaires de cas non évidents.
- `Docs/design-rules.md` : règles de design à appliquer dès la structuration — Single Responsibility à tous les niveaux et architecture en couches avec dépendances strictement descendantes (pas de `utils`/`common`, injection explicite, interfaces définies côté consommateur).

## Objectif

CLI Go (binaire unique, `simu-abonnement`) qui lit une courbe de charge Linky/Enedis (`.xlsx`, pas de 30 min), calcule le coût pour chaque abonnement d'un fichier YAML (Base, HP/HC, Tempo…) part fixe proratisée incluse, et produit un classeur Excel comparatif.

## Architecture prévue (SPEC §7.2)

```
cmd/simu-abonnement/   main : options (flag), codes de retour 0/1/2
internal/enedis/       un xlsx -> loadcurve.Statement{PRM, []Measurement} ; découverte des .xlsx d'un répertoire
internal/loadcurve/    modèle de courbe de charge + fusion de plusieurs Statement (dernier lu écrase, avertissements)
internal/subscription/ types métier Subscription/Tariff/FixedFee + Validate() (couverture 24 h, errors.Join)
internal/config/       YAML -> []subscription.Subscription validés
internal/calendar/     couleurs de jours (CSV date;couleur), couleur par défaut
internal/pricing/      moteur de calcul PUR : aucune E/S, ne connaît ni Excel ni YAML
internal/export/       production du classeur Excel
testdata/
```

Dépendances autorisées : `github.com/xuri/excelize/v2`, `gopkg.in/yaml.v3`, stdlib pour le reste.

## Pièges métier à garder en tête

- La valeur Enedis est une **puissance moyenne en kW**, pas des kWh : `kWh = kW × (Fin − Début)` en heures, sans supposer 0,5 h. Valeur de contrôle sur l'exemple : 336 mesures, 60,379 kWh.
- Ne pas coder en dur les lignes du xlsx : repérer la feuille de données et la ligne d'en-tête `Début/Fin/Valeur`. Les horodatages sont des chaînes `JJ/MM/AAAA HH:MM:SS` (accepter aussi `datetime`).
- Fuseau `Europe/Paris` avec `import _ "time/tzdata"` (Windows). Aux changements d'heure (46/50 créneaux), **ne jamais dédoublonner sur l'horodatage**.
- Tempo : la journée tarifaire commence à `debut_journee_tarifaire` (06:00), donc un créneau à 01:00 prend la couleur de J−1, mais l'agrégation de sortie reste en jour **civil**.
- Plages tarifaires : `debut == fin` = 24 h ; `fin < debut` = à cheval sur minuit ; validation de couverture exacte de 24 h sans chevauchement par abonnement et par couleur ; créneaux à cheval sur une frontière non alignée répartis au prorata.
- `--courbe` est répétable (fichier ou répertoire, non récursif). Fusion : clé = instant absolu de début (pas la chaîne horaire, à cause du changement d'heure), dernier lu gagne même pour `NA`, chevauchement = warning non bloquant, PRM différents = erreur, trous = créneaux manquants. Au-delà de ~16 384 colonnes, la feuille `Détail 30 min` est omise.
- Valeurs `NA` : conservées, marquées manquantes, exclues des calculs, comptées par jour.
- Montants en `float64`, arrondi à l'affichage uniquement. Pas de `panic` pour les erreurs de données (`fmt.Errorf("...: %w")`).
- Sortie Excel (REQ-OUT-02) : colonnes de synthèse à gauche, détail à droite, volets figés via `SetPanes` sur la première colonne de détail.

## Commandes

```
go build ./...            # ou go build -o simu-abonnement ./cmd/simu-abonnement
go vet ./... ; gofmt -l .
go test ./...             # un seul test : go test ./internal/<pkg> -run <NomDuTest>
```

Exemple d'exécution : `simu-abonnement --courbe <fichier.xlsx|répertoire> --abonnements testdata/abonnements.yaml [--couleurs tempo.csv|--couleur-par-defaut bleu] [--detail] --sortie resultat.xlsx`.

Note outillage : sous cet environnement, ne pas écrire de littéral BOM/`\u` dans les fichiers via les outils d'écriture (utiliser `"\xef\xbb\xbf"`).
