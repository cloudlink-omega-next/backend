(function() {
  const baseUrl = '{{ .BaseURL }}';

  // Error responses (rate limits, server errors) are rendered as HTML pages, so
  // the body is read as text and only parsed when possible. Otherwise a failure
  // would reject before any message could be shown.
  function parseResponse(res) {
    return res.text().then(function(text) {
      var data;
      try {
        data = JSON.parse(text);
      } catch (e) {
        data = {
          result: res.status === 429
            ? (typeof t === 'function' ? t('admin_rate_limit') : 'Too many requests, please wait a moment and try again.')
            : (typeof t === 'function' ? t('admin_request_failed') + res.status : 'Request failed with status ' + res.status + '.'),
        };
      }
      return { status: res.status, data: data };
    });
  }

  function unreachable() {
    return { status: 0, data: { result: (typeof t === 'function' ? t('admin_server_unreachable') : 'Could not reach the server.') } };
  }

  function apiGet(path) {
    return fetch(baseUrl + path, { credentials: 'same-origin' }).then(parseResponse).catch(unreachable);
  }

  function apiPut(path, body) {
    return fetch(baseUrl + path, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify(body),
    }).then(parseResponse).catch(unreachable);
  }

  function apiPost(path, body) {
    return fetch(baseUrl + path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify(body || {}),
    }).then(parseResponse).catch(unreachable);
  }

  function apiDelete(path) {
    return fetch(baseUrl + path, {
      method: 'DELETE',
      credentials: 'same-origin',
    }).then(parseResponse).catch(unreachable);
  }

  function showTab(name) {
    document.querySelectorAll('.admin-tab').forEach(function(el) {
      el.classList.add('hidden');
    });
    var target = document.getElementById('tab-' + name);
    if (target) {
      target.classList.remove('hidden');
    }
  }

  function setText(id, value) {
    var el = document.getElementById(id);
    if (!el) return;
    el.textContent = value != null ? value : '-';
  }

  function statusBadge(status) {
    if (status === 'successful') return '<span class="text-green-600">' + (typeof t === 'function' ? t('status_successful') : 'Successful') + '</span>';
    if (status === 'warning') return '<span class="text-yellow-600">' + (typeof t === 'function' ? t('log_warning') : 'Warning') + '</span>';
    if (status === 'failed') return '<span class="text-red-600">' + (typeof t === 'function' ? t('log_failed') : 'Failed') + '</span>';
    return '<span>' + escapeHtml(String(status)) + '</span>';
  }

  function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  function renderLogs(tbodyId, logs) {
    var tbody = document.getElementById(tbodyId);
    if (!tbody) return;
    if (!Array.isArray(logs) || logs.length === 0) {
      tbody.innerHTML = '<tr><td class="px-4 py-2" colspan="4">' + (typeof t === 'function' ? t('admin_no_logs') : 'No logs found.') + '</td></tr>';
      return;
    }
    var html = '';
    logs.forEach(function(log) {
      html += '<tr class="border-b hover:bg-gray-100 dark:hover:bg-gray-700">';
      html += '<td class="px-4 py-2">' + escapeHtml(log.timestamp || '') + '</td>';
      html += '<td class="px-4 py-2">' + escapeHtml(log.class || '') + '</td>';
      html += '<td class="px-4 py-2">' + escapeHtml(log.description || '') + '</td>';
      html += '<td class="px-4 py-2">' + statusBadge(log.status) + '</td>';
      html += '</tr>';
    });
    tbody.innerHTML = html;
  }

  function renderAccounts(accounts) {
    var tbody = document.getElementById('accounts-tab-body');
    if (!tbody) return;
    if (!Array.isArray(accounts) || accounts.length === 0) {
      tbody.innerHTML = '<tr><td class="px-4 py-2" colspan="6">' + (typeof t === 'function' ? t('admin_no_accounts') : 'No accounts found.') + '</td></tr>';
      return;
    }
    var html = '';
    accounts.forEach(function(account) {
      var avatarUrl = account.avatar_url || '/assets/static/img/ui/placeholder_user.png';
      var state = String(account.state ?? '');
      var isBanned = state.indexOf('banned') !== -1 || state.indexOf('1') !== -1;
      var actionButtons = '<button class="px-2 py-1 bg-red-500 text-white rounded text-sm mr-1" onclick="banUser(\'' + account.id + '\')">' + (typeof t === 'function' ? t('admin_ban') : 'Ban') + '</button>' +
        '<button class="px-2 py-1 bg-green-500 text-white rounded text-sm mr-1" onclick="unbanUser(\'' + account.id + '\')">' + (typeof t === 'function' ? t('admin_unban') : 'Unban') + '</button>' +
        '<button class="px-2 py-1 bg-gray-500 text-white rounded text-sm" onclick="deleteUser(\'' + account.id + '\')">' + (typeof t === 'function' ? t('admin_delete_account') : 'Delete') + '</button>';
      if (isBanned) {
        actionButtons = '<button class="px-2 py-1 bg-green-500 text-white rounded text-sm mr-1" onclick="unbanUser(\'' + account.id + '\')">' + (typeof t === 'function' ? t('admin_unban') : 'Unban') + '</button>' +
          '<button class="px-2 py-1 bg-gray-500 text-white rounded text-sm" onclick="deleteUser(\'' + account.id + '\')">' + (typeof t === 'function' ? t('admin_delete_account') : 'Delete') + '</button>';
      }
      html += '<tr class="border-b hover:bg-gray-100 dark:hover:bg-gray-700">';
      html += '<td class="px-4 py-2"><img src="' + escapeHtml(avatarUrl) + '" class="w-8 h-8 rounded-full object-cover" alt="avatar" onerror="this.src=\'/assets/static/img/ui/placeholder_user.png\'" /></td>';
      html += '<td class="px-4 py-2">' + escapeHtml(account.username || '') + '</td>';
      html += '<td class="px-4 py-2">' + escapeHtml(account.email || '') + '</td>';
      html += '<td class="px-4 py-2">' + escapeHtml(state) + '</td>';
      html += '<td class="px-4 py-2">' + escapeHtml(account.created_at || '') + '</td>';
      html += '<td class="px-4 py-2">' + actionButtons + '</td>';
      html += '</tr>';
    });
    tbody.innerHTML = html;
  }

  function renderReports(reports) {
    var tbody = document.getElementById('reports-tab-body');
    if (!tbody) return;
    if (!Array.isArray(reports) || reports.length === 0) {
      tbody.innerHTML = '<tr><td class="px-4 py-2" colspan="6">' + (typeof t === 'function' ? t('admin_no_reports') : 'No reports found.') + '</td></tr>';
      return;
    }
    var html = '';
    reports.forEach(function(report) {
      var reporterName = report.reporter_username || 'Unknown';
      var reportedName = report.reported_username || 'Unknown';
      var reportType = report.report_type || '';
      var details = escapeHtml(report.details || '');
      var createdAt = report.created_at || '';
      html += '<tr class="border-b hover:bg-gray-100 dark:hover:bg-gray-700">';
      html += '<td class="px-4 py-2">' + escapeHtml(reporterName) + '</td>';
      html += '<td class="px-4 py-2">' + escapeHtml(reportedName) + '</td>';
      html += '<td class="px-4 py-2">' + escapeHtml(reportType) + '</td>';
      html += '<td class="px-4 py-2">' + details + '</td>';
      html += '<td class="px-4 py-2">' + escapeHtml(createdAt) + '</td>';
      html += '<td class="px-4 py-2">' +
        '<button class="px-2 py-1 bg-green-500 text-white rounded text-sm mr-1" onclick="resolveReport(\'' + report.id + '\', \'dismiss\')">' + (typeof t === 'function' ? t('admin_resolve_dismiss') : 'Dismiss') + '</button>' +
        '<button class="px-2 py-1 bg-yellow-500 text-white rounded text-sm mr-1" onclick="resolveReport(\'' + report.id + '\', \'warn\')">' + (typeof t === 'function' ? t('admin_resolve_warn') : 'Warn') + '</button>' +
        '<button class="px-2 py-1 bg-red-500 text-white rounded text-sm" onclick="resolveReport(\'' + report.id + '\', \'ban\')">' + (typeof t === 'function' ? t('admin_resolve_ban') : 'Ban') + '</button>' +
        '</td>';
      html += '</tr>';
    });
    tbody.innerHTML = html;
  }

  function renderGames(games) {
    var tbody = document.getElementById('games-tab-body');
    if (!tbody) return;
    if (!Array.isArray(games) || games.length === 0) {
      tbody.innerHTML = '<tr><td class="px-4 py-2" colspan="5">' + (typeof t === 'function' ? t('admin_no_pending_games') : 'No pending games.') + '</td></tr>';
      return;
    }
    var html = '';
    games.forEach(function(game) {
      html += '<tr class="border-b hover:bg-gray-100 dark:hover:bg-gray-700">';
      html += '<td class="px-4 py-2">' + escapeHtml(game.name || '') + '<br /><span class="text-xs text-gray-500 dark:text-gray-400">' + escapeHtml(game.id || '') + '</span></td>';
      html += '<td class="px-4 py-2">' + escapeHtml(game.developer || '') + '<br /><span class="text-xs text-gray-500 dark:text-gray-400">' + escapeHtml(game.developer_id || '') + '</span></td>';
      html += '<td class="px-4 py-2">' + escapeHtml(game.description || '') + '</td>';
      html += '<td class="px-4 py-2">' + escapeHtml(game.created_at || '') + '</td>';
      html += '<td class="px-4 py-2">' +
        '<button id="game-approve-' + game.id + '" class="px-2 py-1 bg-green-500 text-white rounded text-sm mr-1 disabled:opacity-50" onclick="approveGame(\'' + game.id + '\')">' + (typeof t === 'function' ? t('status_approved') : 'Approve') + '</button>' +
        '<button id="game-reject-' + game.id + '" class="px-2 py-1 bg-red-500 text-white rounded text-sm disabled:opacity-50" onclick="rejectGame(\'' + game.id + '\')">' + (typeof t === 'function' ? t('status_denied') : 'Reject') + '</button>' +
        '</td>';
      html += '</tr>';
    });
    tbody.innerHTML = html;
  }

  function setGamesStatus(message, kind) {
    var status = document.getElementById('games-status');
    if (!status) return;
    status.textContent = message || '';
    status.className = 'mt-3 text-sm ' + (
      kind === 'error'
        ? 'text-red-600 dark:text-red-400'
        : kind === 'success'
          ? 'text-green-600 dark:text-green-400'
          : 'text-gray-600 dark:text-gray-300'
    );
  }

  // Publishing moves the whole game package on disk, so the request can take a
  // while. The buttons are locked to prevent duplicate submissions and a status
  // is shown until the server answers.
  function setGameButtonsDisabled(id, disabled) {
    ['game-approve-', 'game-reject-'].forEach(function(prefix) {
      var button = document.getElementById(prefix + id);
      if (button) button.disabled = disabled;
    });
  }

  function reviewGame(id, action) {
    setGameButtonsDisabled(id, true);
    var statusText;
    if (typeof t === 'function') {
      statusText = t(action === 'approve' ? 'admin_publishing' : 'admin_rejecting');
    } else {
      statusText = action === 'approve' ? 'Publishing game, please wait...' : 'Rejecting game, please wait...';
    }
    setGamesStatus(statusText);
    apiPost('/api/v1/admin/games/' + id + '/' + action).then(function(res) {
      setGameButtonsDisabled(id, false);
      if (res.status === 200) {
        setGamesStatus((typeof t === 'function' ? t('admin_done') : 'Done.'), 'success');
        loadGames();
        return;
      }
      var errorText;
      if (typeof t === 'function') {
        errorText = t('admin_load_pending_failed') + action + t('admin_game_delete_failed') + (res.data && res.data.result ? res.data.result : t('error_occurred'));
      } else {
        errorText = 'Failed to ' + action + ' game: ' + (res.data && res.data.result ? res.data.result : 'Unknown error');
      }
      setGamesStatus(errorText, 'error');
    });
  }

  function loadGames() {
    apiGet('/api/v1/admin/games').then(function(res) {
      if (res.status === 200 && res.data && Array.isArray(res.data.data)) {
        renderGames(res.data.data);
        return;
      }
      setGamesStatus((typeof t === 'function' ? t('admin_load_pending_failed') : 'Failed to load pending games: ') + (res.data && res.data.result ? res.data.result : (typeof t === 'function' ? t('error_occurred') : 'Unknown error')), 'error');
    });
  }

  function loadOverview() {
    apiGet('/api/v1/admin/overview').then(function(res) {
      if (res.status !== 200 || !res.data || res.data.result !== 'OK') {
        return;
      }
      var data = res.data.data || {};
      setText('stat-registered-users', data.registered_users);
      setText('stat-active-sessions', data.active_sessions);
      setText('stat-active-lobbies', data.active_lobbies);
      setText('stat-active-voice', data.active_voice_connections);
      setText('stat-published-games', data.published_games);
    });

    apiGet('/api/v1/admin/logs?page=1&limit=10').then(function(res) {
      if (res.status === 200) {
        renderLogs('overview-logs-body', res.data.data || []);
      }
    });
  }

  function loadLogs() {
    apiGet('/api/v1/admin/logs?page=1&limit=50').then(function(res) {
      if (res.status === 200) {
        renderLogs('logs-tab-body', res.data.data || []);
      }
    });
  }

  function loadReports() {
    var typeFilter = document.getElementById('report-type-filter').value || '';
    var url = '/api/v1/admin/reports?page=1&limit=50';
    if (typeFilter) {
      url += '&type=' + encodeURIComponent(typeFilter);
    }
    apiGet(url).then(function(res) {
      if (res.status === 200) {
        renderReports(res.data.data || []);
      }
    });
  }

  function loadAccounts() {
    var search = document.getElementById('account-search').value || '';
    var url = '/api/v1/admin/accounts?page=1&limit=50';
    if (search) {
      url += '&search=' + encodeURIComponent(search);
    }
    apiGet(url).then(function(res) {
      if (res.status === 200) {
        renderAccounts(res.data.data || []);
      }
    });
  }

  function loadStorage() {
    apiGet('/api/v1/admin/storage').then(function(res) {
      if (res.status !== 200 || !res.data || res.data.result !== 'OK') {
        return;
      }
      var data = res.data.data || {};
      setText('stat-hosted-games', data.total_hosted_games);
      setText('stat-cloud-saves', data.total_cloud_saves);
      setText('stat-images', data.total_images);
    });
  }

  function loadSettings() {
    apiGet('/api/v1/admin/settings').then(function(res) {
      if (res.status !== 200 || !res.data || res.data.result !== 'OK') {
        return;
      }
      var data = res.data.data || {};
      var emailInput = document.getElementById('admin-email');
      if (emailInput) {
        emailInput.value = data.admin_email || '';
      }
    });
  }

  function setGameManagementStatus(message, kind) {
    var status = document.getElementById('game-management-status');
    if (!status) return;
    status.textContent = message || '';
    status.className = 'mt-3 text-sm ' + (
      kind === 'error'
        ? 'text-red-600 dark:text-red-400'
        : kind === 'success'
          ? 'text-green-600 dark:text-green-400'
          : 'text-gray-600 dark:text-gray-300'
    );
  }

  function setCloudSaveStatus(message, kind) {
    var status = document.getElementById('cloud-saves-status');
    if (!status) return;
    status.textContent = message || '';
    status.className = 'mt-3 text-sm ' + (
      kind === 'error'
        ? 'text-red-600 dark:text-red-400'
        : kind === 'success'
          ? 'text-green-600 dark:text-green-400'
          : 'text-gray-600 dark:text-gray-300'
    );
  }

  function renderGameManagement(games) {
    var tbody = document.getElementById('game-management-tab-body');
    if (!tbody) return;
    if (!Array.isArray(games) || games.length === 0) {
      tbody.innerHTML = '<tr><td class="px-4 py-2" colspan="5">' + (typeof t === 'function' ? t('admin_game_management_empty') : 'No games found.') + '</td></tr>';
      return;
    }
    var html = '';
    games.forEach(function(game) {
      var state = game.state || 0;
      var isActive = (state & 1) !== 0;
      var isVerified = (state & 2) !== 0;
      var visibility = (isActive && isVerified) ? 'public' : 'private';
      html += '<tr class="border-b hover:bg-gray-100 dark:hover:bg-gray-700">';
      html += '<td class="px-4 py-2">' + escapeHtml(game.name || '') + '<br /><span class="text-xs text-gray-500 dark:text-gray-400">' + escapeHtml(game.id || '') + '</span></td>';
      html += '<td class="px-4 py-2">' + escapeHtml(game.developer || '') + '<br /><span class="text-xs text-gray-500 dark:text-gray-400">' + escapeHtml(game.developer_id || '') + '</span></td>';
      html += '<td class="px-4 py-2">' + escapeHtml(typeof t === 'function' ? t(visibility === 'public' ? 'admin_visibility_public' : 'admin_visibility_private') : visibility) + '</td>';
      html += '<td class="px-4 py-2">' + escapeHtml(game.created_at || '') + '</td>';
      html += '<td class="px-4 py-2">' +
        '<select id="game-visibility-' + game.id + '" class="px-2 py-1 border rounded text-sm mr-1 bg-white dark:bg-gray-700 dark:text-white">' +
        '<option value="public"' + (visibility === 'public' ? ' selected' : '') + '>' + (typeof t === 'function' ? t('admin_visibility_public') : 'Public') + '</option>' +
        '<option value="private"' + (visibility === 'private' ? ' selected' : '') + '>' + (typeof t === 'function' ? t('admin_visibility_private') : 'Private') + '</option>' +
        '</select>' +
        '<button class="px-2 py-1 bg-blue-500 text-white rounded text-sm mr-1" onclick="updateGameVisibility(\'' + game.id + '\')">' + (typeof t === 'function' ? t('admin_update') : 'Update') + '</button>' +
        '<button class="px-2 py-1 bg-yellow-500 text-white rounded text-sm mr-1" onclick="replaceGameFile(\'' + game.id + '\')">' + (typeof t === 'function' ? t('admin_replace') : 'Replace') + '</button>' +
        '<button class="px-2 py-1 bg-red-500 text-white rounded text-sm" onclick="deleteGame(\'' + game.id + '\')">' + (typeof t === 'function' ? t('admin_delete') : 'Delete') + '</button>' +
        '</td>';
      html += '</tr>';
    });
    tbody.innerHTML = html;
  }

  function renderCloudSaves(saves) {
    var tbody = document.getElementById('cloud-saves-tab-body');
    if (!tbody) return;
    if (!Array.isArray(saves) || saves.length === 0) {
      tbody.innerHTML = '<tr><td class="px-4 py-2" colspan="6">No cloud saves found.</td></tr>';
      return;
    }
    var html = '';
    saves.forEach(function(save) {
      html += '<tr class="border-b hover:bg-gray-100 dark:hover:bg-gray-700">';
      html += '<td class="px-4 py-2">' + escapeHtml(String(save.user_id || '')) + '</td>';
      html += '<td class="px-4 py-2">' + escapeHtml(String(save.save_slot || '')) + '</td>';
      html += '<td class="px-4 py-2">' + escapeHtml(save.developer_game_name || save.developer_game_id || '') + '</td>';
      html += '<td class="px-4 py-2"><textarea readonly class="w-full h-24 px-2 py-1 border rounded bg-gray-50 dark:bg-gray-700 dark:text-white text-xs">' + escapeHtml(save.save_data || '') + '</textarea></td>';
      html += '<td class="px-4 py-2">' + escapeHtml(save.updated_at || '') + '</td>';
      html += '<td class="px-4 py-2">' +
        '<button class="px-2 py-1 bg-red-500 text-white rounded text-sm" onclick="deleteCloudSave(\'' + save.user_id + '\', \'' + save.save_slot + '\', \'' + save.developer_game_id + '\')">' + (typeof t === 'function' ? t('admin_cloud_save_delete') : 'Delete') + '</button>' +
        '</td>';
      html += '</tr>';
    });
    tbody.innerHTML = html;
  }

  function loadGameManagement() {
    apiGet('/api/v1/admin/games?all=1').then(function(res) {
      if (res.status === 200 && res.data && Array.isArray(res.data.data)) {
        renderGameManagement(res.data.data);
        return;
      }
      setGameManagementStatus((typeof t === 'function' ? t('admin_load_games_failed') : 'Failed to load games: ') + (res.data && res.data.result ? res.data.result : (typeof t === 'function' ? t('error_occurred') : 'Unknown error')), 'error');
    });
  }

  function loadCloudSaves() {
    apiGet('/api/v1/admin/cloud-saves').then(function(res) {
      if (res.status === 200 && res.data && Array.isArray(res.data.data)) {
        renderCloudSaves(res.data.data);
        return;
      }
      setCloudSaveStatus((typeof t === 'function' ? t('admin_load_cloud_saves_failed') : 'Failed to load cloud saves: ') + (res.data && res.data.result ? res.data.result : (typeof t === 'function' ? t('error_occurred') : 'Unknown error')), 'error');
    });
  }

  window.deleteGame = function(id) {
    if (!confirm((typeof t === 'function' ? t('admin_delete_confirm') : 'Are you sure you want to delete this? This action cannot be undone.'))) {
      return;
    }
    apiDelete('/api/v1/admin/games/' + id).then(function(res) {
      if (res.status === 200) {
        alert((typeof t === 'function' ? t('admin_game_deleted') : 'Game deleted successfully.'));
        loadGameManagement();
      } else {
        alert((typeof t === 'function' ? t('admin_game_delete_failed') : 'Failed to delete game: ') + (res.data && res.data.result ? res.data.result : (typeof t === 'function' ? t('error_occurred') : 'Unknown error')));
      }
    });
  };

  window.updateGameVisibility = function(id) {
    var select = document.getElementById('game-visibility-' + id);
    if (!select) return;
    var visibility = select.value;
    apiPut('/api/v1/admin/games/' + id + '/visibility', { visibility: visibility }).then(function(res) {
      if (res.status === 200) {
        alert((typeof t === 'function' ? t('admin_visibility_updated') : 'Game visibility updated.'));
        loadGameManagement();
      } else {
        alert((typeof t === 'function' ? t('admin_visibility_update_failed') : 'Failed to update visibility: ') + (res.data && res.data.result ? res.data.result : (typeof t === 'function' ? t('error_occurred') : 'Unknown error')));
      }
    });
  };

  window.replaceGameFile = function(id) {
    var input = document.createElement('input');
    input.type = 'file';
    input.accept = '.zip';
    input.onchange = function() {
      if (!input.files || !input.files[0]) return;
      var formData = new FormData();
      formData.append('game', input.files[0]);
      fetch('/api/v1/admin/games/' + id + '/replace', {
        method: 'POST',
        credentials: 'same-origin',
        body: formData
      })
      .then(function(res) { return res.text().then(function(text) {
        var data;
        try { data = JSON.parse(text); } catch (e) { data = { result: (typeof t === 'function' ? t('admin_request_failed') : 'Request failed with status ') + res.status + '.' }; }
        return { status: res.status, data: data };
      }); })
      .then(function(res) {
        if (res.status === 200) {
          alert((typeof t === 'function' ? t('admin_file_replaced') : 'Game file replaced successfully.'));
          loadGameManagement();
        } else {
          alert((typeof t === 'function' ? t('admin_file_replace_failed') : 'Failed to replace file: ') + (res.data && res.data.result ? res.data.result : (typeof t === 'function' ? t('error_occurred') : 'Unknown error')));
        }
      });
    };
    input.click();
  };

  window.deleteCloudSave = function(userId, slot, gameId) {
    if (!confirm((typeof t === 'function' ? t('admin_delete_confirm') : 'Are you sure you want to delete this? This action cannot be undone.'))) {
      return;
    }
    var body = { developer_game_id: gameId };
    apiPost('/api/v1/admin/users/' + userId + '/cloud-saves/' + slot + '/delete', body).then(function(res) {
      if (res.status === 200) {
        alert((typeof t === 'function' ? t('admin_cloud_save_deleted') : 'Cloud save deleted successfully.'));
        loadCloudSaves();
      } else {
        alert((typeof t === 'function' ? t('admin_cloud_save_delete_failed') : 'Failed to delete cloud save: ') + (res.data && res.data.result ? res.data.result : (typeof t === 'function' ? t('error_occurred') : 'Unknown error')));
      }
    });
  };

  window.updateCloudSave = function(userId, slot, gameId) {
    var newData = prompt((typeof t === 'function' ? t('admin_cloud_save_data') : 'Enter new save data:'));
    if (newData === null) return;
    var body = { developer_game_id: gameId, save_data: newData };
    apiPut('/api/v1/admin/users/' + userId + '/cloud-saves/' + slot, body).then(function(res) {
      if (res.status === 200) {
        alert((typeof t === 'function' ? t('admin_cloud_save_updated') : 'Cloud save updated successfully.'));
        loadCloudSaves();
      } else {
        alert((typeof t === 'function' ? t('admin_cloud_save_update_failed') : 'Failed to update cloud save: ') + (res.data && res.data.result ? res.data.result : (typeof t === 'function' ? t('error_occurred') : 'Unknown error')));
      }
    });
  };

  window.banUser = function(id) {
    apiPost('/api/v1/admin/ban/' + id).then(function(res) {
      if (res.status === 200) {
        alert((typeof t === 'function' ? t('admin_user_banned') : 'User banned successfully.'));
        loadAccounts();
      } else {
        alert((typeof t === 'function' ? t('admin_user_ban_failed') : 'Failed to ban user: ') + (res.data && res.data.result ? res.data.result : (typeof t === 'function' ? t('error_occurred') : 'Unknown error')));
      }
    });
  };

  window.unbanUser = function(id) {
    apiPost('/api/v1/admin/unban/' + id).then(function(res) {
      if (res.status === 200) {
        alert((typeof t === 'function' ? t('admin_user_unbanned') : 'User unbanned successfully.'));
        loadAccounts();
      } else {
        alert((typeof t === 'function' ? t('admin_user_unban_failed') : 'Failed to unban user: ') + (res.data && res.data.result ? res.data.result : (typeof t === 'function' ? t('error_occurred') : 'Unknown error')));
      }
    });
  };

  window.deleteUser = function(id) {
    if (!confirm((typeof t === 'function' ? t('admin_confirm_delete_account') : 'Are you sure you want to delete this account? This action cannot be undone.'))) {
      return;
    }
    apiPost('/api/v1/admin/delete/' + id).then(function(res) {
      if (res.status === 200) {
        alert((typeof t === 'function' ? t('admin_account_deleted') : 'Account deleted successfully.'));
        loadAccounts();
      } else {
        alert((typeof t === 'function' ? t('admin_account_delete_failed') : 'Failed to delete user: ') + (res.data && res.data.result ? res.data.result : (typeof t === 'function' ? t('error_occurred') : 'Unknown error')));
      }
    });
  };

  window.resolveReport = function(id, action) {
    apiPost('/api/v1/admin/reports/' + id + '/resolve', { action: action }).then(function(res) {
      if (res.status === 200) {
        alert((typeof t === 'function' ? t('admin_report_resolved') : 'Report resolved successfully.'));
        loadReports();
      } else {
        alert((typeof t === 'function' ? t('admin_report_resolve_failed') : 'Failed to resolve report: ') + (res.data && res.data.result ? res.data.result : (typeof t === 'function' ? t('error_occurred') : 'Unknown error')));
      }
    });
  };

  window.approveGame = function(id) {
    reviewGame(id, 'approve');
  };

  window.rejectGame = function(id) {
    if (!confirm((typeof t === 'function' ? t('admin_reject_confirm') : 'Are you sure you want to reject this game? The uploaded package will be deleted.'))) {
      return;
    }
    reviewGame(id, 'reject');
  };

  document.querySelectorAll('[data-tab]').forEach(function(link) {
    link.addEventListener('click', function(e) {
      e.preventDefault();
      var tab = link.getAttribute('data-tab');
      showTab(tab);
      if (tab === 'overview') loadOverview();
      if (tab === 'logs') loadLogs();
      if (tab === 'accounts') loadAccounts();
      if (tab === 'reports') loadReports();
      if (tab === 'games') loadGames();
      if (tab === 'game_management') loadGameManagement();
      if (tab === 'cloud_saves') loadCloudSaves();
      if (tab === 'storage') loadStorage();
      if (tab === 'settings') loadSettings();
    });
  });

  var searchTimer;
  var searchInput = document.getElementById('account-search');
  if (searchInput) {
    searchInput.addEventListener('input', function() {
      clearTimeout(searchTimer);
      searchTimer = setTimeout(loadAccounts, 300);
    });
  }

  var reportTypeFilter = document.getElementById('report-type-filter');
  if (reportTypeFilter) {
    reportTypeFilter.addEventListener('change', function() {
      loadReports();
    });
  }

  var saveButton = document.getElementById('admin-settings-save');
  if (saveButton) {
    saveButton.addEventListener('click', function() {
      var emailInput = document.getElementById('admin-email');
      var status = document.getElementById('admin-settings-status');
      var payload = { admin_email: emailInput ? emailInput.value : '' };
      apiPut('/api/v1/admin/settings', payload).then(function(res) {
        if (res.status === 200) {
          if (status) status.textContent = (typeof t === 'function' ? t('admin_save_success') : 'Saved.');
        } else {
          if (status) status.textContent = (typeof t === 'function' ? t('admin_save_failed') : 'Save failed.');
        }
      });
    });
  }

  applyI18n();
  showTab('overview');
  loadOverview();
})();
</script>
