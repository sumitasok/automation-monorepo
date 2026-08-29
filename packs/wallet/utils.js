// JSONL Parsing - skip metadata header on line 1
function parseJSONL(text) {
  const lines = text.trim().split('\n');
  const records = [];
  
  for (let i = 1; i < lines.length; i++) {  // Skip first line (metadata)
    if (!lines[i].trim()) continue;
    try {
      records.push(JSON.parse(lines[i]));
    } catch (e) {
      console.warn(`Failed to parse line ${i + 1}:`, e.message);
    }
  }
  return records;
}

// Format date to readable string
function formatDate(isoString) {
  if (!isoString) return 'N/A';
  try {
    const date = new Date(isoString);
    return date.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  } catch {
    return isoString;
  }
}

// Format amount with currency and sign
function formatAmount(transaction) {
  if (!transaction || !transaction.amount) return 'N/A';
  const value = transaction.amount.value;
  const code = transaction.amount.currencyCode || 'INR';
  const sign = value < 0 ? '−' : '+';
  const absValue = Math.abs(value).toLocaleString('en-IN', {
    maximumFractionDigits: 2,
    minimumFractionDigits: 2
  });
  return `${sign}${absValue} ${code}`;
}

// Debounce function for search input
function debounce(fn, delay = 300) {
  let timeoutId;
  return function debounced(...args) {
    clearTimeout(timeoutId);
    timeoutId = setTimeout(() => fn.apply(this, args), delay);
  };
}

// Get value from nested object path (e.g., "amount.value")
function getNestedValue(obj, path) {
  return path.split('.').reduce((current, key) => current?.[key], obj);
}

// Truncate text with ellipsis
function truncate(text, length = 50) {
  if (!text) return '';
  return text.length > length ? text.substring(0, length) + '...' : text;
}

// Check if two dates overlap with range
function isDateInRange(date, startDate, endDate) {
  const d = new Date(date);
  if (startDate) {
    const start = new Date(startDate);
    start.setHours(0, 0, 0, 0);
    if (d < start) return false;
  }
  if (endDate) {
    const end = new Date(endDate);
    end.setHours(23, 59, 59, 999);
    if (d > end) return false;
  }
  return true;
}

// Check if amount is in range
function isAmountInRange(amount, minAmount, maxAmount) {
  if (minAmount !== null && minAmount !== undefined && amount < minAmount) return false;
  if (maxAmount !== null && maxAmount !== undefined && amount > maxAmount) return false;
  return true;
}
