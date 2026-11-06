# MyanKafe Finance

Native iOS and Android app for daily Finance work: income, expense, transfer, review, and reports. It is a separate application from MyanKafe Staff (`com.myankafe.staff`). It is not a WebView.

## Architecture

- Expo 57, React Native 0.86, React 19, Expo Router
- TanStack Query for server state. Cached figures are not the books of record.
- Bearer sessions in Expo SecureStore. Selected entity public ID is the only accounting preference kept in AsyncStorage.
- One API client talks to the existing `/api/v1` routes. There is no `/mobile` API.

```text
app/            Expo Router screens
src/api         HTTP client and unauthorized handling
src/auth        SecureStore session and entity preference
src/lib         decimal strings, entry/transfer rules, production host guard
src/components  screen primitives
```

## Identity

| | |
|---|---|
| Name | MyanKafe Finance |
| Slug | `myankafe-finance` |
| Scheme | `myankafefinance` |
| iOS bundle | `com.myankafe.finance` |
| Android package | `com.myankafe.finance` |

## API configuration

```text
EXPO_PUBLIC_APP_ENV=development|preview|production
EXPO_PUBLIC_API_BASE_URL=https://finance-api.myankafe.com/api/v1
```

Copy `.env.example` to `.env` for local Metro. The documented production API is `https://finance-api.myankafe.com` (`docs/DEPLOYMENT_ACCEPTANCE_REPORT.md`). A production build fails closed unless the base URL is exactly `https://finance-api.myankafe.com/api/v1`. Localhost, loopback, emulator hosts, staging hosts, and plain HTTP are rejected.

Login sends `X-Session-Transport: bearer`. The `session_token` is stored only in SecureStore and sent as `Authorization: Bearer`. A 401 clears that token and returns to Login. Password change asks for bearer transport and stores the replacement token. Cookie login for the website is unchanged.

## Development

```bash
cd mobile-app
npm install
npm start
```

Point `EXPO_PUBLIC_API_BASE_URL` at a disposable Finance API. Do not post test entries to production.

## Testing

```bash
npm run typecheck
npm test
```

## Android and iOS

```bash
npx expo prebuild --platform android --no-install
npx expo prebuild --platform ios --no-install
npx expo run:android
npx expo run:ios
```

`eas.json` has `development`, `preview`, and `production` profiles. No signing keys, keystores, or EAS tokens belong in git.

## EAS

This app does not use the Staff EAS project. No Finance project ID is committed. Create one yourself:

```bash
cd mobile-app
npx eas login
npx eas init
```

`eas init` writes the new project ID into the Expo config. Do not paste the Staff project ID.

## Security and storage

| SecureStore | AsyncStorage |
|---|---|
| session token | selected entity public ID |
| session expiry | |

Not stored: passwords, journals, balances, reports, attachments, or offline drafts. Mutations are disabled while offline and are never queued.

Amounts stay decimal strings. The client does not convert currencies.

## V1 scope

Home, transactions, new income/expense (draft and post), transfer, six reports, account ledger, profile, sessions, and password change.

Attachment flow matches the website: create the draft, upload files, and post only if every upload succeeds. A failed upload leaves the draft unposted.

## Intentionally web-only

Users and roles, entity creation, chart of accounts, financial account setup, currencies, exchange rates, opening balances, accounting start date, lock and unlock, integrations, sales-channel setup, contact-type setup, inter-entity setup, system configuration, and manual journals. Contacts can be searched for an entry. Creating a missing contact stays on the website.
