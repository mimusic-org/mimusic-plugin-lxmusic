// API 基础路径
const API_BASE = '/api/v1/plugin/lxmusic/api';

// 当前状态
let currentSources = [];
let searchResults = [];
let currentPage = 1;
let totalResults = 0;
let currentSourceId = '';
let currentKeyword = '';

/**
 * 从 localStorage 获取认证 Token
 */
function getAuthToken() {
    try {
        var authData = localStorage.getItem('mimusic-auth');
        if (authData) {
            var auth = JSON.parse(authData);
            return auth.accessToken || '';
        }
    } catch (e) {
        console.error('获取 Token 失败:', e);
    }
    return '';
}

/**
 * 获取认证头
 */
function getAuthHeaders(isJson = true) {
    var headers = {};
    if (isJson) {
        headers['Content-Type'] = 'application/json';
    }
    var token = getAuthToken();
    if (token) {
        headers['Authorization'] = 'Bearer ' + token;
    }
    return headers;
}

/**
 * 显示 Toast 消息
 */
function showToast(message, type = 'info') {
    const toast = document.getElementById('toast');
    toast.textContent = message;
    toast.className = 'toast ' + type + ' show';
    
    setTimeout(() => {
        toast.className = 'toast';
    }, 3000);
}

/**
 * 格式化时长为 mm:ss
 */
function formatDuration(seconds) {
    if (!seconds || seconds <= 0) return '--:--';
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
}

/**
 * HTML 转义
 */
function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// ============ 音源管理 ============

/**
 * 加载音源列表
 */
async function loadSources() {
    try {
        const response = await fetch(`${API_BASE}/sources`, {
            headers: getAuthHeaders()
        });
        const result = await response.json();
        
        if (result.success) {
            currentSources = result.data || [];
            renderSources();
            updateSourceSelect();
        } else {
            showToast('加载音源失败: ' + (result.message || result.error || '未知错误'), 'error');
        }
    } catch (error) {
        console.error('加载音源失败:', error);
        showToast('加载音源失败: ' + error.message, 'error');
    }
}

/**
 * 渲染音源列表
 */
function renderSources() {
    const container = document.getElementById('sourceList');
    
    if (currentSources.length === 0) {
        container.innerHTML = '<div class="empty-state"><div class="icon">📦</div><div>暂无音源，请导入音源脚本</div></div>';
        return;
    }
    
    let html = '';
    for (const source of currentSources) {
        html += `
            <div class="source-item" data-id="${escapeHtml(source.id)}">
                <div class="source-info">
                    <div class="source-name">${escapeHtml(source.name)}</div>
                    <div class="source-meta">
                        版本: ${escapeHtml(source.version || '-')} | 
                        作者: ${escapeHtml(source.author || '-')} | 
                        导入时间: ${source.imported_at ? new Date(source.imported_at).toLocaleString() : '-'}
                    </div>
                </div>
                <div class="source-actions">
                    <button class="btn btn-danger btn-small" onclick="deleteSource('${escapeHtml(source.id)}')">删除</button>
                </div>
            </div>
        `;
    }
    container.innerHTML = html;
}

/**
 * 更新音源下拉选择
 */
function updateSourceSelect() {
    const select = document.getElementById('sourceSelect');
    const searchBtn = document.getElementById('searchBtn');
    
    if (currentSources.length === 0) {
        select.innerHTML = '<option value="">请先导入音源</option>';
        searchBtn.disabled = true;
    } else {
        let html = '<option value="">请选择音源</option>';
        for (const source of currentSources) {
            html += `<option value="${escapeHtml(source.id)}">${escapeHtml(source.name)}</option>`;
        }
        select.innerHTML = html;
        searchBtn.disabled = false;
    }
}

/**
 * 导入音源
 */
async function importSource(file) {
    const formData = new FormData();
    formData.append('file', file);
    
    try {
        showToast('正在导入...', 'info');
        
        const headers = {};
        const token = getAuthToken();
        if (token) {
            headers['Authorization'] = 'Bearer ' + token;
        }
        
        const response = await fetch(`${API_BASE}/sources/import`, {
            method: 'POST',
            headers: headers,
            body: formData
        });
        const result = await response.json();
        
        if (result.success) {
            showToast('导入成功', 'success');
            loadSources();
        } else {
            showToast('导入失败: ' + (result.message || result.error || '未知错误'), 'error');
        }
    } catch (error) {
        console.error('导入音源失败:', error);
        showToast('导入失败: ' + error.message, 'error');
    }
}

/**
 * 删除音源
 */
async function deleteSource(id) {
    if (!confirm('确定要删除这个音源吗？')) {
        return;
    }
    
    try {
        const response = await fetch(`${API_BASE}/sources?id=${encodeURIComponent(id)}`, {
            method: 'DELETE',
            headers: getAuthHeaders()
        });
        const result = await response.json();
        
        if (result.success) {
            showToast('删除成功', 'success');
            loadSources();
        } else {
            showToast('删除失败: ' + (result.message || result.error || '未知错误'), 'error');
        }
    } catch (error) {
        console.error('删除音源失败:', error);
        showToast('删除失败: ' + error.message, 'error');
    }
}

// ============ 歌曲搜索 ============

/**
 * 搜索歌曲
 */
async function search(keyword, sourceId, page = 1) {
    if (!keyword.trim()) {
        showToast('请输入搜索关键词', 'warning');
        return;
    }
    
    if (!sourceId) {
        showToast('请选择音源', 'warning');
        return;
    }
    
    currentKeyword = keyword;
    currentSourceId = sourceId;
    currentPage = page;
    
    const searchBtn = document.getElementById('searchBtn');
    searchBtn.disabled = true;
    searchBtn.innerHTML = '<span class="loading"></span>搜索中...';
    
    try {
        const params = new URLSearchParams({
            keyword: keyword,
            source_id: sourceId,
            page: page
        });
        
        const response = await fetch(`${API_BASE}/search?${params}`, {
            headers: getAuthHeaders()
        });
        const result = await response.json();
        
        if (result.success) {
            searchResults = result.data.list || [];
            totalResults = result.data.total || 0;
            renderResults();
            document.getElementById('resultSection').style.display = 'block';
        } else {
            showToast('搜索失败: ' + (result.message || result.error || '未知错误'), 'error');
        }
    } catch (error) {
        console.error('搜索失败:', error);
        showToast('搜索失败: ' + error.message, 'error');
    } finally {
        searchBtn.disabled = false;
        searchBtn.textContent = '搜索';
    }
}

/**
 * 渲染搜索结果
 */
function renderResults() {
    const container = document.getElementById('resultList');
    const countEl = document.getElementById('resultCount');
    
    countEl.textContent = `(共 ${totalResults} 条)`;
    
    if (searchResults.length === 0) {
        container.innerHTML = '<div class="empty-state"><div class="icon">🔍</div><div>没有找到相关歌曲</div></div>';
        return;
    }
    
    let html = '';
    for (let i = 0; i < searchResults.length; i++) {
        const song = searchResults[i];
        html += `
            <div class="result-item">
                <input type="checkbox" class="result-checkbox" data-index="${i}" onchange="updateSelectedCount()">
                <div class="result-info">
                    <div class="result-name">${escapeHtml(song.name)}</div>
                    <div class="result-meta">${escapeHtml(song.singer)} - ${escapeHtml(song.album || '未知专辑')}</div>
                </div>
                <div class="result-duration">${formatDuration(song.duration)}</div>
            </div>
        `;
    }
    container.innerHTML = html;
    
    // 重置全选状态
    document.getElementById('selectAll').checked = false;
    updateSelectedCount();
    
    // 渲染分页
    renderPagination();
}

/**
 * 渲染分页
 */
function renderPagination() {
    const container = document.getElementById('pagination');
    const totalPages = Math.ceil(totalResults / 30);
    
    if (totalPages <= 1) {
        container.innerHTML = '';
        return;
    }
    
    let html = `
        <button ${currentPage <= 1 ? 'disabled' : ''} onclick="search('${escapeHtml(currentKeyword)}', '${escapeHtml(currentSourceId)}', ${currentPage - 1})">上一页</button>
        <span class="page-info">第 ${currentPage} / ${totalPages} 页</span>
        <button ${currentPage >= totalPages ? 'disabled' : ''} onclick="search('${escapeHtml(currentKeyword)}', '${escapeHtml(currentSourceId)}', ${currentPage + 1})">下一页</button>
    `;
    container.innerHTML = html;
}

/**
 * 全选/反选
 */
function toggleSelectAll() {
    const checked = document.getElementById('selectAll').checked;
    const checkboxes = document.querySelectorAll('.result-checkbox');
    checkboxes.forEach(cb => cb.checked = checked);
    updateSelectedCount();
}

/**
 * 更新已选数量
 */
function updateSelectedCount() {
    const checkboxes = document.querySelectorAll('.result-checkbox:checked');
    const count = checkboxes.length;
    document.getElementById('selectedCount').textContent = `已选 ${count} 首`;
    document.getElementById('importSongsBtn').disabled = count === 0;
}

// ============ 批量导入 ============

/**
 * 导入选中的歌曲
 */
async function importSelectedSongs() {
    const checkboxes = document.querySelectorAll('.result-checkbox:checked');
    if (checkboxes.length === 0) {
        showToast('请选择要导入的歌曲', 'warning');
        return;
    }
    
    const songs = [];
    checkboxes.forEach(cb => {
        const index = parseInt(cb.dataset.index);
        const song = searchResults[index];
        songs.push({
            name: song.name,
            singer: song.singer,
            album: song.album,
            source: song.source,
            music_id: song.music_id,
            quality: '320k',
            img: song.img
        });
    });
    
    // 显示进度区域
    const progressSection = document.getElementById('importProgress');
    const progressFill = document.getElementById('progressFill');
    const progressText = document.getElementById('progressText');
    const importResults = document.getElementById('importResults');
    
    progressSection.style.display = 'block';
    progressFill.style.width = '0%';
    progressText.textContent = '正在导入...';
    importResults.innerHTML = '';
    
    // 禁用导入按钮
    const importBtn = document.getElementById('importSongsBtn');
    importBtn.disabled = true;
    importBtn.textContent = '导入中...';
    
    try {
        const response = await fetch(`${API_BASE}/songs/import`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({
                source_id: currentSourceId,
                songs: songs
            })
        });
        const result = await response.json();
        
        if (result.success) {
            const data = result.data;
            progressFill.style.width = '100%';
            progressText.textContent = `导入完成: 成功 ${data.success} 首, 失败 ${data.failed} 首`;
            
            // 显示详细结果
            let html = '';
            for (const item of data.results) {
                if (item.success) {
                    html += `<div class="import-result-item success">✓ ${escapeHtml(item.name)}</div>`;
                } else {
                    html += `<div class="import-result-item error">✗ ${escapeHtml(item.name)}: ${escapeHtml(item.error)}</div>`;
                }
            }
            importResults.innerHTML = html;
            
            if (data.success > 0) {
                showToast(`成功导入 ${data.success} 首歌曲`, 'success');
            }
            if (data.failed > 0) {
                showToast(`${data.failed} 首歌曲导入失败`, 'error');
            }
        } else {
            progressText.textContent = '导入失败: ' + (result.message || result.error || '未知错误');
            showToast('导入失败: ' + (result.message || result.error || '未知错误'), 'error');
        }
    } catch (error) {
        console.error('导入失败:', error);
        progressText.textContent = '导入失败: ' + error.message;
        showToast('导入失败: ' + error.message, 'error');
    } finally {
        importBtn.disabled = false;
        importBtn.textContent = '导入选中歌曲';
        updateSelectedCount();
    }
}

// ============ 初始化 ============

document.addEventListener('DOMContentLoaded', function() {
    // 加载音源列表
    loadSources();
    
    // 导入音源按钮
    document.getElementById('importBtn').addEventListener('click', function() {
        document.getElementById('sourceFile').click();
    });
    
    // 文件选择
    document.getElementById('sourceFile').addEventListener('change', function(e) {
        const file = e.target.files[0];
        if (file) {
            importSource(file);
            e.target.value = ''; // 清空以允许重复选择同一文件
        }
    });
    
    // 搜索按钮
    document.getElementById('searchBtn').addEventListener('click', function() {
        const keyword = document.getElementById('keyword').value;
        const sourceId = document.getElementById('sourceSelect').value;
        search(keyword, sourceId, 1);
    });
    
    // 回车搜索
    document.getElementById('keyword').addEventListener('keypress', function(e) {
        if (e.key === 'Enter') {
            const keyword = document.getElementById('keyword').value;
            const sourceId = document.getElementById('sourceSelect').value;
            search(keyword, sourceId, 1);
        }
    });
    
    // 全选
    document.getElementById('selectAll').addEventListener('change', toggleSelectAll);
    
    // 导入歌曲按钮
    document.getElementById('importSongsBtn').addEventListener('click', importSelectedSongs);
});
