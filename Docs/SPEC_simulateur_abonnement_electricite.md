# Spécification – Simulateur de coût d'abonnement électrique à partir de la courbe de charge Enedis

## 1. Contexte et objectif

Le titulaire d'un compteur Linky peut télécharger depuis son espace client Enedis sa **courbe de charge** au pas de 30 minutes, au format `.xlsx`.

L'objectif est de développer un programme qui :

1. lit un ou plusieurs fichiers Enedis (fichiers listés ou répertoire) et les fusionne en une seule série (§2.1.1) ;
2. convertit chaque mesure de 30 minutes en énergie consommée (kWh) ;
3. applique à chaque créneau le prix du kWh correspondant, pour **chacun des abonnements** décrits dans un fichier de configuration (Base, Heures Pleines / Heures Creuses, Tempo, ou tout autre tarif à plages horaires) ;
4. ajoute la part fixe (abonnement) proratisée ;
5. produit un **fichier Excel** présentant la consommation puis, sur les lignes suivantes, le coût selon chaque abonnement, au niveau du jour et au total sur la période.

L'usage visé est la comparaison d'offres : « avec ma consommation réelle sur cette période, combien m'aurait coûté chaque abonnement ? »

## 2. Données d'entrée

### 2.1 Fichier de courbe de charge Enedis (.xlsx)

Exemple fourni : `22184080962179_Export_courbe_de_charge_Consommation_01092026-08092026.xlsx`

Structure observée :

| Élément | Valeur observée |
|---|---|
| Feuilles | `Page d'accueil` (texte d'information, à ignorer) et `Puissance moyenne soutirée` (données) |
| En-tête de métadonnées | Lignes 6 à 11 de la feuille de données : PRM (cellule C8), type de comptage, date de début (E10), date de fin (E11) |
| Ligne d'en-tête du tableau | Ligne 16 : `Début` (col. C), `Fin` (col. D), `Valeur (en kW)` (col. E) |
| Première ligne de données | Ligne 17 |
| Format des horodatages | **Chaînes de caractères** `JJ/MM/AAAA HH:MM:SS` (heure légale française, sans fuseau) |
| Format des valeurs | Nombre décimal (float) |
| Volume de l'exemple | 7 jours × 48 créneaux = 336 lignes (du 01/09/2026 00:00 au 08/09/2026 00:00) |

Exigences de lecture :

- **REQ-IN-01** : ne pas coder en dur les numéros de ligne. Localiser la feuille de données (par son nom, ou à défaut la feuille contenant l'en-tête `Début` / `Fin` / `Valeur`), puis repérer la ligne d'en-tête et lire les lignes suivantes jusqu'à la première ligne vide.
- **REQ-IN-02** : parser les horodatages au format `%d/%m/%Y %H:%M:%S`. Accepter aussi des cellules déjà typées `datetime` (robustesse face à une ouverture/réenregistrement du fichier dans Excel).
- **REQ-IN-03** : le fichier peut contenir des valeurs indisponibles (la légende indique `NA` = « Données indisponibles »). Ces créneaux sont conservés, marqués comme manquants, exclus des calculs, et comptabilisés dans un rapport (nombre de créneaux manquants par jour).
- **REQ-IN-04** : extraire le PRM et les dates de début/fin de chaque fichier. Dans le fichier de sortie sont reportés le PRM et la période **de la série fusionnée** (premier `Début` / dernier `Fin` des mesures), ainsi que la liste des fichiers sources. Les jours de la période (et donc la part fixe, REQ-CALC-04) se déduisent des mesures, jamais des dates de métadonnées (la date de fin d'un export est exclusive : 01/09 → 08/09 = 7 jours).

### 2.1.1 Fichiers multiples et fusion

La commande accepte **plusieurs sources**, chacune étant un fichier `.xlsx` ou un répertoire. Toutes les mesures lues sont rassemblées comme si elles provenaient d'un seul fichier.

- **REQ-IN-08** : une source peut être un fichier ou un répertoire. Un répertoire est parcouru **sans récursion** ; seuls les fichiers `*.xlsx` sont retenus (les autres fichiers et les fichiers temporaires Excel `~$*.xlsx` sont ignorés). Les fichiers d'un répertoire sont traités dans l'ordre alphabétique ; les sources de la ligne de commande dans l'ordre donné. Un répertoire sans `.xlsx`, ou aucune mesure au total, est une erreur d'entrée (code 1).
- **REQ-IN-09** : l'ordre de lecture ainsi défini est l'ordre de priorité : en cas de doublon, **la mesure lue en dernier écrase la précédente**, sans contrôle de cohérence entre les deux versions. Cela vaut aussi pour une valeur manquante (`NA`) qui écrase une valeur valide, et inversement.
- **REQ-IN-10** : identité d'une mesure = son **instant de début** (`Début` converti en instant absolu, voir REQ-TIME-01). Deux mesures de même instant sont des doublons ; une même source ne doit pas se doublonner elle-même sans que cela soit dû au changement d'heure.
- **REQ-IN-11** : tout chevauchement produit un **avertissement non bloquant** (stderr et feuille `Paramètres`) : pour chaque couple (fichier écrasant, fichier écrasé), nombre de créneaux en doublon et plage concernée. Le code de retour reste 0.
- **REQ-IN-12** : si les PRM diffèrent entre fichiers, c'est une **erreur bloquante** (code 1) : fusionner deux compteurs n'a pas de sens. Les trous entre fichiers (créneaux absents de la série fusionnée) sont signalés par un avertissement et comptés comme créneaux manquants, comme les `NA` (REQ-IN-03).

### 2.2 ⚠️ Point d'attention : unité de la valeur (kW ≠ kWh)

La valeur fournie par Enedis est une **puissance moyenne en kW** sur le créneau, et non une énergie. La feuille d'accueil le précise : la courbe de charge représente la puissance moyenne soutirée, exprimée en kW.

- **REQ-IN-05** : l'énergie d'un créneau se calcule par `énergie_kWh = puissance_kW × durée_h`, où la durée est calculée à partir de `Fin − Début` (0,5 h pour un créneau standard). Ne jamais supposer un pas fixe de 30 minutes.

Valeur de contrôle sur le fichier d'exemple : somme des puissances × 0,5 = **60,379 kWh** sur les 7 jours.

### 2.3 Fichier de configuration des abonnements

Format : **YAML** (lisible et éditable à la main). Un fichier contient une liste d'abonnements à comparer.

Modèle de données :

```text
Abonnement
├── nom                : str           (ex. "EDF Base 6 kVA")
├── puissance_kva      : int, optionnel (informatif, voir §5.4)
├── part_fixe
│   ├── montant        : float (€ TTC)
│   └── periodicite    : "annuelle" | "mensuelle"
├── debut_journee_tarifaire : "HH:MM", optionnel, défaut "00:00" (voir §4.3)
└── tarifs             : liste de Tarif
    Tarif
    ├── couleur        : str, optionnel ("*" ou absent = tous les jours ; sinon "bleu", "blanc", "rouge", …)
    ├── libelle        : str (ex. "HP", "HC", "Base")
    ├── debut          : "HH:MM"
    ├── fin            : "HH:MM"  (peut être < debut : plage à cheval sur minuit, ex. 22:00 → 06:00)
    └── prix_kwh       : float (€ TTC / kWh)
```

Exemple de configuration (les prix ci-dessous sont **des valeurs d'exemple non contractuelles**, à remplacer par la grille tarifaire réelle en vigueur) :

```yaml
abonnements:
  - nom: "Base 9 kVA"
    puissance_kva: 9
    part_fixe: { montant: 200.00, periodicite: annuelle }
    tarifs:
      - { libelle: Base, debut: "00:00", fin: "00:00", prix_kwh: 0.20 }

  - nom: "HP/HC 9 kVA"
    puissance_kva: 9
    part_fixe: { montant: 17.00, periodicite: mensuelle }
    tarifs:
      - { libelle: HC, debut: "22:00", fin: "06:00", prix_kwh: 0.16 }
      - { libelle: HP, debut: "06:00", fin: "22:00", prix_kwh: 0.21 }

  - nom: "Tempo 9 kVA"
    puissance_kva: 9
    part_fixe: { montant: 210.00, periodicite: annuelle }
    debut_journee_tarifaire: "06:00"
    tarifs:
      - { couleur: bleu,  libelle: HC, debut: "22:00", fin: "06:00", prix_kwh: 0.13 }
      - { couleur: bleu,  libelle: HP, debut: "06:00", fin: "22:00", prix_kwh: 0.16 }
      - { couleur: blanc, libelle: HC, debut: "22:00", fin: "06:00", prix_kwh: 0.14 }
      - { couleur: blanc, libelle: HP, debut: "06:00", fin: "22:00", prix_kwh: 0.19 }
      - { couleur: rouge, libelle: HC, debut: "22:00", fin: "06:00", prix_kwh: 0.15 }
      - { couleur: rouge, libelle: HP, debut: "06:00", fin: "22:00", prix_kwh: 0.70 }
```

Convention : `debut == fin` (ex. `00:00` → `00:00`) signifie « toute la journée ».

### 2.4 Calendrier des couleurs de jours (Tempo)

Les abonnements à couleurs nécessitent de connaître la couleur de chaque journée de la période.

- **REQ-IN-13** : le calendrier ne sert qu'à choisir, pour chaque créneau, le jeu de tarifs d'un abonnement à couleurs. Un abonnement sans couleur (tarifs sans `couleur`, ou `"*"`) l'ignore et s'applique tous les jours ; il est donc directement comparable à un abonnement à couleurs sur les mêmes mesures. Le calendrier (`--couleurs`) n'est exigé que si au moins un abonnement utilise des couleurs, et la ligne « Couleur Tempo » de la synthèse n'apparaît que dans ce cas.
- **REQ-IN-14** : pour une couleur donnée, les tarifs applicables sont ceux de cette couleur plus ceux marqués `"*"`/sans couleur ; la vérification de couverture exacte sur 24 h (REQ-CALC-02) se fait sur cet ensemble, pour chaque couleur déclarée dans l'abonnement.

- **REQ-IN-06** : le calendrier est fourni dans un fichier CSV `date;couleur` (ex. `2026-09-01;bleu`).
- **REQ-IN-07** : si une journée de la période n'a pas de couleur alors qu'un abonnement en a besoin, le programme s'arrête avec un message listant les dates manquantes, **sauf** si l'option `--couleur-par-defaut <couleur>` est fournie (utile pour une estimation rapide, ex. tout en bleu). Dans ce cas, le fichier de sortie signale les jours dont la couleur a été supposée.
- **Évolution possible (hors V1)** : récupération automatique des couleurs historiques via une API publique (RTE ou service tiers), avec mise en cache locale dans le CSV.

## 3. Traitements

### 3.1 Calcul de l'énergie par créneau

Pour chaque ligne : `kWh = kW × (Fin − Début) en heures`.

### 3.2 Affectation d'un prix à chaque créneau

Pour chaque abonnement et chaque créneau :

1. Déterminer la **journée tarifaire** du créneau (voir §4.3) puis sa **couleur** via le calendrier (si l'abonnement utilise des couleurs).
2. Sélectionner le tarif dont la couleur correspond et dont la plage horaire contient l'heure de début du créneau (si le créneau chevauche une frontière de plage, voir REQ-CALC-01).
3. `coût_variable = kWh × prix_kwh`.

- **REQ-CALC-01** : un créneau à cheval sur deux plages tarifaires (cas d'une frontière non alignée sur la demi-heure, ex. HC débutant à 22:15) est **réparti au prorata du temps** passé dans chaque plage, en supposant une puissance constante sur le créneau.
- **REQ-CALC-02** : au chargement de la configuration, vérifier pour chaque abonnement et chaque couleur que les plages **couvrent exactement 24 h sans chevauchement**. Toute incohérence bloque l'exécution avec un message explicite (abonnement, couleur, plage en cause).

### 3.3 Part fixe

- **REQ-CALC-03** : la part fixe est proratisée par jour :
  - périodicité annuelle : `montant / nombre de jours de l'année civile concernée` (365 ou 366) ;
  - périodicité mensuelle : `montant / nombre de jours du mois concerné`.
- **REQ-CALC-04** : la part fixe est imputée à chaque jour **civil** de la période, y compris les jours partiellement couverts ou contenant des données manquantes.

### 3.4 Agrégations

Pour chaque abonnement : coût variable par jour, part fixe par jour, coût total par jour, puis totaux sur la période. Ajouter une **extrapolation annuelle indicative** : `coût moyen journalier × 365`, en signalant clairement qu'elle est non représentative de la saisonnalité si la période est courte.

## 4. Règles de gestion temporelles

### 4.1 Fuseau horaire

Les horodatages Enedis sont en heure légale française. Le programme travaille en heure locale `Europe/Paris` (`time.LoadLocation("Europe/Paris")`). Le binaire embarque la base des fuseaux horaires (`import _ "time/tzdata"`) pour fonctionner aussi sur un poste Windows sans base tzdata.

### 4.2 Changements d'heure

- Le jour du passage à l'heure d'été compte 46 créneaux, celui du passage à l'heure d'hiver 50.
- **REQ-TIME-01** : vérifier comment Enedis représente l'heure dupliquée lors du passage à l'heure d'hiver (créneaux en double `02:00–02:30`). Les deux créneaux doivent être conservés et comptés, même s'ils ont des horodatages identiques. Ne pas dédoublonner sur l'horodatage affiché : l'horodatage local ambigu est résolu en instant absolu selon l'ordre d'apparition dans le fichier (1re occurrence = heure d'été, 2e = heure d'hiver). La fusion multi-fichiers (REQ-IN-10) dédoublonne sur cet instant absolu, jamais sur la chaîne. Limite connue : un fichier ne contenant qu'une seule occurrence d'un horodatage ambigu est interprété comme heure d'été.
- **REQ-TIME-02** : prévoir un test unitaire avec un fichier synthétique couvrant chaque changement d'heure.

### 4.3 Journée tarifaire vs journée civile

Pour Tempo, la couleur d'un jour s'applique de **6 h ce jour-là à 6 h le lendemain**. Exemple : le créneau du 02/09 à 01:00 relève de la couleur du **01/09**.

- **REQ-TIME-03** : le champ `debut_journee_tarifaire` de l'abonnement détermine la journée tarifaire utilisée pour retrouver la couleur. Le regroupement par jour dans le fichier de sortie reste en **jour civil** (00:00–24:00), pour que tous les abonnements soient comparables colonne à colonne.
- Conséquence : le premier créneau 00:00–06:00 de la période nécessite la couleur de la veille de la date de début (à prévoir dans le calendrier, ou couverte par `--couleur-par-defaut`).

## 5. Données de sortie : fichier Excel

### 5.1 Principe de présentation (demande explicite)

La consommation apparaît **en ligne**, et les coûts figurent **sur les lignes suivantes, une ligne par abonnement**. Les colonnes représentent le temps.

**REQ-OUT-02 – Ordre des colonnes : synthèse à gauche, détail à droite.** Sur chaque feuille de données, les colonnes de synthèse (Total période, Moyenne / jour, Extrapolation annuelle, écart avec le moins cher…) sont placées **immédiatement après la colonne des libellés et avant les colonnes de détail** (jours ou créneaux). Les colonnes de libellés et de synthèse sont **figées** (volet figé, avec excelize `SetPanes` positionné sur la première colonne de détail et la première ligne de données). En faisant défiler horizontalement, l'utilisateur garde ainsi en permanence sous les yeux les libellés et les valeurs globales, tandis que le détail défile à droite. Une bordure verticale marquée sépare la zone de synthèse de la zone de détail.

### 5.2 Feuilles

**Feuille `Synthèse journalière`** (feuille principale)

| | Total période | Moyenne / jour | Extrapolation annuelle | Écart vs moins cher | 01/09/2026 | 02/09/2026 | … |
|---|---|---|---|---|---|---|---|
| Consommation (kWh) | 60,38 | … | … | | 8,52 | … | … |
| Base 9 kVA (€) | … | … | … | … | … | … | … |
| HP/HC 9 kVA (€) | … | … | … | … | … | … | … |
| Tempo 9 kVA (€) | … | … | … | … | … | … | … |
| Couleur Tempo | | | | | bleu | … | … |

Colonnes figées : A à E (libellés et synthèse). Le détail journalier commence en colonne F.

- Sous chaque ligne d'abonnement, ajouter éventuellement (option `--detail`) des sous-lignes : part fixe, coût variable, et kWh par libellé de tarif (HP, HC, …).
- Mettre en évidence (mise en forme conditionnelle) l'abonnement le moins cher sur le total de la période. La colonne « Écart vs moins cher » indique l'écart en € et en % par rapport à lui.

**Feuille `Détail 30 min`**

Même logique, avec une colonne par créneau de 30 minutes : ligne d'en-tête « Début », ligne « Puissance moyenne (kW) », ligne « Énergie (kWh) », puis une ligne de coût variable par abonnement (et une ligne indiquant le libellé de tarif appliqué, option `--detail`). Conformément à REQ-OUT-02, les colonnes de synthèse (Total période, Moyenne par créneau, Maximum) précèdent les colonnes de créneaux et sont figées avec la colonne des libellés. Pour un mois, cela représente environ 1 500 colonnes, ce qui reste dans les limites d'Excel (16 384 colonnes, soit environ 340 jours au pas de 30 min).
- **REQ-OUT-03** : si la série fusionnée dépasse cette limite, la feuille `Détail 30 min` n'est pas générée, un avertissement l'indique, et les autres feuilles sont produites normalement (l'option `--sans-detail-30min` permet aussi de la désactiver explicitement).

**Feuille `Paramètres`**

Rappel de la configuration utilisée (abonnements, plages, prix, part fixe), du PRM, de la période, des fichiers sources (dans l'ordre de lecture), de la date de génération, ainsi que des anomalies : créneaux manquants (dont trous entre fichiers), chevauchements entre fichiers (REQ-IN-11), couleurs supposées.

### 5.3 Règles de forme

- Nombres en format français (`#,##0.00 €`, `#,##0.000` pour les kWh), police Arial.
- **REQ-OUT-01** : en V1, les valeurs sont calculées par le programme Go et écrites comme valeurs. Les colonnes de totaux peuvent utiliser des formules (`=SOMME(...)`).
- **Évolution possible** : génération entièrement en formules, à partir de la feuille `Paramètres`, pour permettre de modifier un prix directement dans Excel et voir le résultat se recalculer.

### 5.4 Information sur la puissance souscrite (optionnel)

Afficher par jour la **puissance moyenne maximale sur 30 min** observée, comparée à la `puissance_kva` de chaque abonnement. Avertissement à faire figurer : une moyenne sur 30 minutes sous-estime les pointes instantanées, donc cette donnée est indicative et ne permet pas de garantir qu'une puissance inférieure suffirait.

## 6. Interface

Programme en ligne de commande :

```bash
simu-abonnement \
  --courbe 22184080962179_Export_courbe_de_charge_Consommation_01092026-08092026.xlsx \
  [--courbe autre.xlsx] [--courbe ./repertoire_exports] \
  --abonnements abonnements.yaml \
  --couleurs tempo.csv \
  [--couleur-par-defaut bleu] \
  [--detail] \
  [--sans-detail-30min] \
  --sortie resultat.xlsx
```

`--courbe` est **répétable** et accepte un fichier `.xlsx` ou un répertoire (REQ-IN-08) ; au moins une occurrence est requise. Avec `flag`, l'option répétable s'implémente via un `flag.Value` personnalisé.

Codes de retour : `0` succès (avertissements éventuels sur stderr, dont les chevauchements de fichiers), `1` erreur de données d'entrée (fichier illisible, configuration invalide, couleurs manquantes, PRM différents), `2` erreur inattendue. Un résumé (coût total par abonnement, abonnement le moins cher) est aussi affiché sur la console.

## 7. Exigences techniques

### 7.1 Langage et dépendances

- **Go ≥ 1.25** (version du `go.mod`), module `elecsubscribesimul`. Livrable : un **binaire unique** autonome, compilable pour Linux, macOS et Windows (`GOOS`/`GOARCH`).
- Dépendances limitées au strict nécessaire :
  - `github.com/xuri/excelize/v2` : lecture du fichier Enedis et production du classeur de sortie (styles, formats numériques, volets figés, mise en forme conditionnelle) ;
  - `gopkg.in/yaml.v3` : lecture de la configuration des abonnements ;
  - bibliothèque standard pour le reste : `flag` pour la ligne de commande (ou `cobra` si des sous-commandes apparaissent), `encoding/csv` pour le calendrier des couleurs, `time` pour les dates.
- Validation de la configuration : pas de bibliothèque de validation imposée. Chaque type expose une méthode `Validate() error` qui renvoie des erreurs explicites, regroupées avec `errors.Join` pour signaler toutes les anomalies en une seule exécution.

### 7.2 Organisation du code

Organisation conforme aux conventions Go, chaque paquet étant testable indépendamment :

```text
elecsubscribesimul/
├── cmd/simu-abonnement/main.go   point d'entrée, parsing des options, codes de retour
├── internal/enedis/              un fichier xlsx → loadcurve.Statement{PRM, Measurements []Measurement{Start, End time.Time; KW float64; Missing bool}} ; découverte des .xlsx d'un répertoire
├── internal/loadcurve/           modèle de courbe de charge ; fusion de plusieurs Statement en une série triée (écrasement, chevauchements, trous) → Curve + []string d'avertissements
├── internal/config/              YAML → []subscription.Subscription validés
├── internal/calendar/           couleurs des jours (CSV), couleur par défaut
├── internal/pricing/            moteur de calcul pur : aucune entrée/sortie, aucune dépendance externe
├── internal/export/              production du classeur Excel
└── testdata/                     fichier Enedis d'exemple, fichiers synthétiques, configurations de test
```

- Le moteur `tarification` reçoit des mesures, des abonnements et un calendrier, et renvoie des structures de résultats (par créneau, par jour, par période). Il ne connaît ni Excel ni YAML.
- Les heures de plage (`"22:00"`) sont converties à la lecture en un type dédié (minutes depuis minuit), pour éviter toute manipulation de chaînes dans le moteur.
- Erreurs : pas de `panic` pour les erreurs de données. Les erreurs sont enrichies avec `fmt.Errorf("...: %w", err)` (fichier, ligne, abonnement concerné).

### 7.3 Calculs numériques

- Montants et énergies manipulés en `float64`, avec arrondi uniquement à l'affichage. Si une exactitude au centime sur les totaux devient nécessaire, passer à `github.com/shopspring/decimal`.

### 7.4 Qualité et tests

- `go vet` et `gofmt` sans avertissement ; `golangci-lint` recommandé.
- Tests avec `go test`, sous forme de **tests table-driven**. Le fichier Enedis d'exemple est placé dans `testdata/`. Les fichiers synthétiques (changements d'heure, valeurs `NA`) sont générés dans les tests avec excelize. Cas à couvrir au minimum :
  - lecture du fichier d'exemple : 336 mesures, total 60,379 kWh ;
  - fusion : deux fichiers contigus (série continue), chevauchants (avertissement, dernier lu gagne, y compris `NA`), séparés par un trou (avertissement, créneaux manquants), PRM différents (erreur), répertoire (ordre alphabétique, non-`.xlsx` et `~$*.xlsx` ignorés), chevauchement au changement d'heure d'hiver (créneaux dupliqués non confondus) ;
  - tarif Base : coût variable = total kWh × prix ;
  - HP/HC : affectation correcte des créneaux 21:30, 22:00, 05:30 et 06:00 ;
  - Tempo : le créneau du jour J à 01:00 prend la couleur de J−1 ;
  - prorata d'un créneau à cheval sur une frontière de plage non alignée ;
  - validation de configuration : plages qui se chevauchent et trou de couverture ;
  - jours de changement d'heure (46 et 50 créneaux) ;
  - valeurs `NA` ;
  - part fixe annuelle (année bissextile) et mensuelle (mois de 28, 30 et 31 jours).
- Un test de bout en bout exécute la commande sur le fichier d'exemple, relit le classeur produit avec excelize et vérifie la présence des feuilles, l'ordre des colonnes (REQ-OUT-02), les volets figés et les totaux.

## 8. Hors périmètre (V1)

- Récupération automatique des données depuis Enedis ou Hellowatt.
- Récupération automatique des couleurs Tempo.
- Prise en compte de l'injection ou de la production solaire.
- Détail des taxes : les prix saisis sont considérés comme TTC, toutes composantes incluses.
- Interface graphique.

## 9. Questions ouvertes

1. ~~Plusieurs fichiers Enedis en entrée~~ : **tranché**, voir §2.1.1 (fichiers multiples ou répertoire, fusion, avertissement sur chevauchement, dernier lu gagne).
2. Faut-il ajouter une feuille de synthèse **mensuelle** pour les périodes de plusieurs mois ?
3. Prévoir des abonnements de type « jours spécifiques » (ex. heures creuses le week-end) : ajouter un critère `jours_semaine` au modèle `Tarif` ?
