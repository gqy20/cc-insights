function formatNumber(value) {
    return Number(value || 0).toLocaleString();
}

function shortModelName(value) {
    if (!value) return 'unknown';
    return value.length > 22 ? value.slice(0, 22) + '...' : value;
}

function shortToolName(value) {
    if (!value) return 'unknown';
    return value.replace(/^mcp__/, '').replace(/__/g, '/').slice(0, 28);
}

function shortEventName(value) {
    if (!value) return 'unknown';
    return value.replace(/^attachment:/, 'att:').slice(0, 28);
}

function shortAgentID(value) {
    if (!value) return 'unknown';
    if (value === 'main') return 'main';
    return value.slice(0, 10);
}

function shortPath(value) {
    if (!value) return 'unknown';
    const parts = value.split('/');
    const name = parts[parts.length - 1] || value;
    return name.length > 30 ? name.slice(0, 30) + '...' : name;
}

function escapeHtml(value) {
    return String(value || '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}
