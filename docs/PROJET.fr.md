# TextDock : vision et point de départ

**Un Mailpit du SMS aujourd'hui, un service d'équipe hébergé demain.**

Le principe : une utilisation légère, sans limiter l'ambition fonctionnelle.
Un binaire ou un conteneur, une UI intégrée, une base locale et un port par défaut
peu courant : **18257**. L'open source local reste autonome.

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
