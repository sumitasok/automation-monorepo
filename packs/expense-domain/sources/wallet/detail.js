function showDetailModal(recordId) {
  const record = appState.getRecord(recordId);
  if (!record) return;

  const modal = document.getElementById('detailModal');
  const content = document.getElementById('detailContent');

  // Build detail content
  const html = `
    <h2>Transaction Details</h2>
    
    <div class="detail-field">
      <div class="detail-label">Date</div>
      <div class="detail-value">${formatDate(record.recordDate)}</div>
    </div>

    <div class="detail-field">
      <div class="detail-label">Amount</div>
      <div class="detail-value" style="font-size: 16px; font-weight: bold;">
        ${formatAmount(record)}
      </div>
    </div>

    <div class="detail-field">
      <div class="detail-label">Counterparty</div>
      <div class="detail-value">${record.counterParty || 'N/A'}</div>
    </div>

    <div class="detail-field">
      <div class="detail-label">Category</div>
      <div class="detail-value">
        ${record.category?.name || 'Uncategorized'}
        ${record.category?.group?.name ? ` (${record.category.group.name})` : ''}
      </div>
    </div>

    <div class="detail-field">
      <div class="detail-label">Account</div>
      <div class="detail-value">${record.account?.name || 'N/A'}</div>
    </div>

    <div class="detail-field">
      <div class="detail-label">State</div>
      <div class="detail-value">${record.recordState || 'N/A'}</div>
    </div>

    ${record.labels && record.labels.length > 0 ? `
      <div class="detail-field">
        <div class="detail-label">Labels</div>
        <div class="detail-tags">
          ${record.labels.map(label => `
            <span class="tag" style="background: ${label.color || '#ecf0f1'}">${label.name}</span>
          `).join('')}
        </div>
      </div>
    ` : ''}

    ${record.notes ? `
      <div class="detail-field">
        <div class="detail-label">Notes</div>
        <div class="detail-value">${record.notes}</div>
      </div>
    ` : ''}

    <div class="detail-field">
      <div class="detail-label">Created</div>
      <div class="detail-value">${formatDate(record.createdAt)}</div>
    </div>

    ${record.updatedAt ? `
      <div class="detail-field">
        <div class="detail-label">Last Updated</div>
        <div class="detail-value">${formatDate(record.updatedAt)}</div>
      </div>
    ` : ''}

    <div class="detail-field">
      <div class="detail-label">ID</div>
      <div class="detail-value" style="font-family: monospace; font-size: 12px; word-break: break-all;">
        ${record.id}
      </div>
    </div>
  `;

  content.innerHTML = html;
  modal.style.display = 'flex';
}

function closeDetailModal() {
  const modal = document.getElementById('detailModal');
  modal.style.display = 'none';
  appState.selectRecord(null);
}

// Attach modal event listeners
document.addEventListener('DOMContentLoaded', () => {
  const modal = document.getElementById('detailModal');
  const closeBtn = document.getElementById('modalCloseBtn');

  closeBtn?.addEventListener('click', closeDetailModal);

  // Close on backdrop click
  modal?.addEventListener('click', (e) => {
    if (e.target === modal) {
      closeDetailModal();
    }
  });

  // Close on ESC key
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && modal?.style.display === 'flex') {
      closeDetailModal();
    }
  });
});
