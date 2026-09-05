# Expense Domain Reports

Report generators for the expense tracking domain.

## Report Types

- **expense-summary**: Monthly/annual expense summary by category
- **category-breakdown**: Pie chart showing expense distribution
- **monthly-trends**: Line chart showing expense trends over time
- **source-analysis**: Analysis of data quality from each source
- **reconciliation**: Reconciliation status of expense entries

## Usage

```javascript
const reports = require('./index.js');
const summary = await reports.expenseSummary({ startDate, endDate });
```

## Implementation Notes

Each report:
- Reads from domain engine output in `~/automation-monorepo-config/data/expense-domain/engine/`
- Applies domain rules during aggregation
- Returns structured data (JSON, CSV, or visualization data)
- Can be exported to PDF, CSV, or displayed in UI
