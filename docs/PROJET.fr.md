# TextDock : vision et point de départ

**Aperçu v0.7 :** labo d'émulateurs Android, sélection explicite de la cible ADB,
injection de SMS simulés et historique. La passerelle Android ajoute le choix
de la SIM et les rapports de présence/modèle/version. La bêta 2 ajoute une boîte
mobile installable et des alertes Web Push génériques, optionnelles et chiffrées.
Les modems USB et les validations physiques multi-SIM/opérateurs et notifications
en arrière-plan restent à réaliser. La bêta 3 ajoute des exemples Android
Retriever/User Consent et iOS `.oneTimeCode`, avec saisie manuelle et tests de
formulaire sur émulateur/simulateur.

**v0.6 en bêta :** relais intégré Twilio, passerelle Android avec APK de
développement, clés d'idempotence, limites d'envoi, expiration et reçus signés.
La compilation et les tests de protocole sont vérifiés ; réception réelle et
autocomplétion restent à valider sur des téléphones physiques. Le site stable
reste sur v0.5.2 avec une présentation technique et des polices Arial/Helvetica.

**Un Mailpit du SMS aujourd'hui, un service d'équipe hébergé demain.**

Le principe : une utilisation légère, sans limiter l'ambition fonctionnelle.
Un binaire ou un conteneur, une UI intégrée, une base locale et un port par défaut
peu courant : **18257**. L'open source local reste autonome.

Le site public et sa documentation sont disponibles sur
**https://fless-lab.github.io/TextDock/**, avec les téléchargements de chaque
release stable. GitHub Pages publie la documentation ; ce n'est pas encore le
service SMS hébergé prévu dans la roadmap.

## Où en est le projet ?

Le socle local est utilisable : captures, téléphone en lecture seule, scénarios,
webhooks, CLI et SDK. Le relais SMS et le labo d'émulateurs sont disponibles en
bêta. Les prochaines étapes comprennent les validations physiques et les modems,
puis les fonctions d'équipe et le SaaS.

## Les numéros de téléphone

TextDock ne génère ni n'attribue actuellement de numéros réels. Tu choisis un
destinataire pour adresser tes messages de test, retrouver un utilisateur,
restreindre une boîte mobile ou appliquer un scénario. En local, ce numéro n'a
pas besoin d'une SIM. Un futur générateur de fixtures ne créerait que des données
fictives, jamais un numéro joignable sur le réseau mobile.

Pour recevoir un vrai SMS dans l'application Messages, il faut le numéro réel du
téléphone et un fournisseur ou une passerelle avec SIM. Les codes OTP, eux, sont
générés et vérifiés par ton application ; TextDock les capture et les extrait.
Voir [les numéros et OTP de test](TEST-NUMBERS.md).

**Avancement v0.2 :** l'appairage QR, les sessions mobiles limitées à un
destinataire/test, les événements en direct et la révocation sont implémentés.
La v0.1 est publiée sur GitHub avec binaires et image Docker via la CI.

**Avancement v0.3 :** projets/inboxes, pagination, favoris, tags, filtres,
exports et commandes CLI sont disponibles, avec sauvegarde/restauration SQLite,
rétention optionnelle, extraction OTP configurable et vérifications d'accessibilité.

**Avancement v0.4 :** scénarios déterministes, statuts simulés, messages entrants,
callbacks signés, retries persistants et inspection/rejeu des tentatives. Les
versions v0.1, v0.2 et v0.3 sont publiées avec CI verte.

**Avancement v0.5 :** un SDK Node commun permet de choisir local, Twilio, Vonage
ou OVH par configuration. Les adaptateurs de capture et leurs limites sont
documentés et testés ; la validation avec comptes fournisseurs réels et téléphone
physique reste à faire. Le SDK est distribué dans les releases GitHub.

## Ce que contient la v0.1.0

La capture fonctionne réellement : API JSON, petit sous-ensemble Twilio testé,
boîte responsive, recherche, export JSON, suppression, encodage/segments SMS,
détection d'OTP et attente de code dans les tests. SQLite conserve les messages.
Le téléphone peut ouvrir l'interface sur le Wi-Fi avec un jeton serveur partagé.

L'autocomplétion native et l'envoi de vrais SMS demandent une couche différente :
un fournisseur, un Android passerelle avec SIM ou un modem cellulaire. La boîte
web seule ne déclenche pas la détection SMS d'Android/iOS. Les tests sur émulateur
Android constituent un troisième parcours, à distinguer du téléphone physique.

## Ordre de construction

1. **v0.1** : capture locale et fondation des releases.
2. **v0.2** : appairage QR, sessions mobiles restreintes, événements en direct.
3. **v0.3** : projets, conversations, recherche avancée, CLI et confort quotidien.
4. **v0.4** : scénarios, erreurs, délais, callbacks et messages entrants simulés.
5. **v0.5** : fidélité fournisseurs et changement local/production.
6. **v0.6** : relais réels, réception SMS native et parcours autofill.
7. **v0.7** : labo Android, modem, exemples iOS et notifications optionnelles.
8. **v0.8** : auto-hébergement en équipe et isolation des accès.
9. **v0.9** : bêta hébergée, organisations, quotas et facturation.
10. **v1.0** : contrats stables, performances et distribution consolidées.

Chaque version a ses critères de validation dans [ROADMAP.md](ROADMAP.md).
On fait des commits cohérents pendant la version, puis un tag annoté à sa fin.
La CI du tag relance les tests avant de publier binaires, checksums et image
Docker multiarchitecture. La publication nécessite un dépôt GitHub distant.

## Architecture

Go + SQLite + React/TypeScript. Un monolithe modulaire, avec des frontières
documentées pour les messages, le stockage, les appareils, les scénarios,
les connecteurs et le futur hébergement. Les modules à venir sont conçus dans la
documentation et ajoutés avec leur premier cas d'usage réel.

Le nom TextDock est un nom de travail ; sa disponibilité reste à vérifier avant
le lancement public. La licence initiale du code est MIT.
