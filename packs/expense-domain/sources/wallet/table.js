class TableRenderer {
  constructor() {
    this.tableBody = document.getElementById('tableBody');
    this.resultCount = document.getElementById('resultCount');
  }

  render(records) {
    if (!records || records.length === 0) {
      this.tableBody.innerHTML = '<tr><td colspan="5" class="no-results">No records found</td></tr>';
      this.updateResultCount(0);
      return;
    }

    const html = records.map(record => {
      const amount = record.amount?.value || 0;
      const amountStr = formatAmount(record);
      const date = formatDate(record.recordDate);
      const counterParty = record.counterParty || 'N/A';
      const category = record.category?.name || 'Uncategorized';
      const account = record.account?.name || 'N/A';

      return `
        <tr data-record-id="${record.id}" class="record-row">
          <td>${date}</td>
          <td>${truncate(counterParty, 40)}</td>
          <td class="amount ${amount < 0 ? 'negative' : 'positive'}">${amountStr}</td>
          <td>${truncate(category, 30)}</td>
          <td>${truncate(account, 30)}</td>
        </tr>
      `;
    }).join('');

    this.tableBody.innerHTML = html;
    this.updateResultCount(records.length);

    // Add click listeners
    this.tableBody.querySelectorAll('.record-row').forEach(row => {
      row.addEventListener('click', (e) => {
        const recordId = row.dataset.recordId;
        appState.selectRecord(recordId);
        showDetailModal(recordId);
      });
    });
  }

  updateResultCount(count) {
    const total = appState.records.length;
    if (count === total) {
      this.resultCount.textContent = `${count} records`;
    } else {
      this.resultCount.textContent = `${count} of ${total} records`;
    }
  }
}

const tableRenderer = new TableRenderer();

// Add styling for table states
const style = document.createElement('style');
style.textContent = `
  .records-table .amount {
    font-weight: 500;
    font-family: 'Courier New', monospace;
  }
  .records-table .amount.negative {
    color: #c0392b;
  }
  .records-table .amount.positive {
    color: #27ae60;
  }
  .state-badge {
    display: inline-block;
    padding: 4px 8px;
    border-radius: 3px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    background: #ecf0f1;
    color: #2c3e50;
  }
  .records-table .no-results {
    text-align: center;
    padding: 40px 12px;
    color: #999;
  }
`;
document.head.appendChild(style);
