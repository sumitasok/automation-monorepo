# Expense Domain UI

Domain-specific user interface for expense tracking.

## Structure

```
ui/
├── components/          # React components
│   ├── TransactionList.jsx
│   ├── TransactionEditor.jsx
│   ├── SourceStatus.jsx
│   └── RulesDisplay.jsx
├── pages/              # Page components
│   ├── Dashboard.jsx
│   ├── Transactions.jsx
│   ├── Sources.jsx
│   └── Rules.jsx
├── lib/
│   └── api-client.js   # API client for domain engine
├── index.html
├── package.json
├── manifest.yaml       # UI manifest (declares requirements)
└── styles/            # CSS/styling
```

## API Integration

The UI communicates with the Domain Engine API at:
- `GET /api/expense-domain/expenses`
- `PATCH /api/expense-domain/expenses/{id}`
- `GET /api/expense-domain/rules`
- `POST /api/expense-domain/jobs/{job}/trigger`

## Getting Started

```bash
cd packs/expense-domain/ui
npm install
npm start
```

## Features

- [ ] Transaction list with filtering/sorting
- [ ] Transaction editor with validation
- [ ] Source status display
- [ ] Rule management UI
- [ ] File upload interface
- [ ] Job status monitoring
