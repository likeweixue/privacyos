<template>
  <div class="pos-page">
    <PageHeader :title="t('photos.title')" :subtitle="t('photos.subtitle')">
      <el-button :icon="Aim" :loading="selectMode" :type="selectMode ? 'primary' : 'default'" @click="toggleSelect">
        {{ t('photos.selectPhotos') }}
      </el-button>
      <el-button :icon="Camera" @click="pickPhotos">{{ t('photos.upload') }}</el-button>
      <el-button :icon="Refresh" :loading="scanning" @click="scan">{{ t('photos.scan') }}</el-button>
      <el-button type="primary" :icon="FolderAdd" @click="openCreateAlbum">{{ t('photos.newAlbum') }}</el-button>
    </PageHeader>

    <div class="filter-bar">
      <el-tabs v-model="tab" class="tabs">
        <el-tab-pane :label="t('photos.timeline')" name="timeline" />
        <el-tab-pane :label="t('photos.albums')" name="albums" />
        <el-tab-pane :label="t('photos.map')" name="map" />
      </el-tabs>
      <div class="pos-spacer" />
      <template v-if="tab === 'timeline'">
        <el-select v-model="filterYear" :placeholder="t('movies.year')" clearable size="small" style="width: 110px" @change="onFilterChange">
          <el-option v-for="y in years" :key="y" :label="String(y)" :value="y" />
        </el-select>
        <el-select v-model="filterMonth" :placeholder="t('photos.month')" clearable size="small" style="width: 100px" @change="onFilterChange">
          <el-option v-for="m in 12" :key="m" :label="String(m)" :value="m" />
        </el-select>
      </template>
      <span class="pos-hint">{{ t('photos.photoCount', { n: total }) }}</span>
    </div>

    <!-- selection toolbar -->
    <div v-if="selectMode" class="select-bar pos-panel">
      <span>{{ t('files.selected', { n: selectedIds.length }) }}</span>
      <div class="pos-spacer" />
      <el-button size="small" :disabled="!selectedIds.length" @click="openAddToAlbum()">
        {{ t('photos.addToAlbum') }}
      </el-button>
      <el-button size="small" type="danger" plain :disabled="!selectedIds.length" @click="deleteSelected">
        {{ t('common.delete') }}
      </el-button>
      <el-button size="small" @click="selectMode = false">{{ t('common.cancel') }}</el-button>
    </div>

    <!-- timeline -->
    <template v-if="tab === 'timeline'">
      <div v-if="albumView" class="album-head pos-panel">
        <el-button link :icon="ArrowLeft" @click="closeAlbum">{{ t('common.back') }}</el-button>
        <strong>{{ albumView.name }}</strong>
        <span class="pos-hint">{{ t('photos.photoCount', { n: albumPhotos.length }) }}</span>
        <div class="pos-spacer" />
        <el-button size="small" type="danger" plain @click="deleteAlbum">{{ t('common.delete') }}</el-button>
      </div>

      <div v-if="!loading && !photos.length" class="pos-panel">
        <EmptyState :title="t('photos.empty')" :description="t('photos.emptyHint')" icon="Picture">
          <el-button type="primary" :icon="Camera" @click="pickPhotos">{{ t('photos.upload') }}</el-button>
        </EmptyState>
      </div>

      <div v-else class="timeline">
        <section v-for="group in groupedPhotos" :key="group.key" class="month-group">
          <div class="month-head">
            <span class="month">{{ group.label }}</span>
            <span class="pos-hint">{{ t('photos.photoCount', { n: group.items.length }) }}</span>
          </div>
          <div class="photo-grid">
            <button
              v-for="photo in group.items"
              :key="photo.id"
              type="button"
              class="photo-cell"
              :class="{ selected: selectedIds.includes(photo.id) }"
              @click="onPhotoClick(photo)"
            >
              <img :src="thumbURL(photo, 400)" :alt="photo.file_name" loading="lazy" />
              <span class="photo-overlay">
                <span>{{ formatDate(photo.taken_at, 'MM-DD HH:mm') }}</span>
                <el-checkbox v-if="selectMode" :model-value="selectedIds.includes(photo.id)" @click.stop @change="toggleSelectPhoto(photo.id)" />
              </span>
            </button>
          </div>
        </section>
        <div class="load-more">
          <el-button v-if="photos.length < total" :loading="loading" @click="loadMore">{{ t('photos.loadingMore') }}</el-button>
          <span v-else-if="photos.length" class="pos-hint">{{ t('photos.allLoaded') }}</span>
        </div>
      </div>
    </template>

    <!-- albums -->
    <template v-else-if="tab === 'albums'">
      <div v-if="!albums.length" class="pos-panel">
        <EmptyState :title="t('photos.noAlbums')" :description="t('photos.emptyHint')" icon="FolderAdd">
          <el-button type="primary" :icon="FolderAdd" @click="openCreateAlbum">{{ t('photos.newAlbum') }}</el-button>
        </EmptyState>
      </div>
      <div v-else class="album-grid">
        <button v-for="album in albums" :key="album.id" type="button" class="album-card pos-panel" @click="openAlbum(album)">
          <div class="album-cover">
            <img v-if="album.cover_photo" :src="thumbURL({ id: album.cover_photo }, 400)" alt="" loading="lazy" />
            <el-icon v-else :size="28"><Folder /></el-icon>
          </div>
          <div class="album-meta">
            <strong>{{ album.name }}</strong>
            <span class="pos-hint">{{ t('photos.photoCount', { n: album.photo_count }) }}</span>
          </div>
          <el-button link type="danger" :icon="Delete" @click.stop="removeAlbum(album)" />
        </button>
      </div>
    </template>

    <!-- map -->
    <template v-else>
      <div class="pos-panel map-panel">
        <div class="panel-head">
          <div>
            <div class="pos-card-title">{{ t('photos.map') }}</div>
            <div class="pos-hint">{{ t('photos.mapHint') }}</div>
          </div>
          <el-tag effect="plain">{{ t('photos.photoCount', { n: geoPhotos.length }) }}</el-tag>
        </div>
        <EChart v-if="geoPhotos.length" :option="mapOption" :height="380" />
        <EmptyState v-else :title="t('photos.noGps')" icon="Location" />
        <div class="geo-list">
          <div v-for="p in geoPhotos" :key="p.id" class="geo-row">
            <el-icon color="#2dd4bf"><Location /></el-icon>
            <span>{{ p.file_name }}</span>
            <span class="pos-hint">{{ p.gps_lat.toFixed(4) }}, {{ p.gps_lng.toFixed(4) }}</span>
            <span class="pos-hint">{{ formatDate(p.taken_at) }}</span>
          </div>
        </div>
      </div>
    </template>

    <input ref="photoInput" type="file" accept="image/*" multiple hidden @change="onPhotosPicked" />

    <!-- photo detail drawer -->
    <el-drawer v-model="detailOpen" :title="detail?.file_name || t('photos.viewPhoto')" size="480px">
      <div v-if="detail" class="detail">
        <el-image :src="fileURL(detail)" fit="contain" class="detail-image" :preview-src-list="[fileURL(detail)]" preview-teleported />
        <div class="detail-actions">
          <el-button :icon="Download" @click="downloadPhoto(detail)">{{ t('photos.downloadOriginal') }}</el-button>
          <el-button :icon="Share" @click="openShare(detail)">{{ t('photos.sharePhoto') }}</el-button>
          <el-button :icon="FolderAdd" @click="openAddToAlbum(detail)">{{ t('photos.addToAlbum') }}</el-button>
          <el-button type="danger" plain :icon="Delete" @click="removePhoto(detail)">{{ t('common.delete') }}</el-button>
        </div>
        <el-descriptions :column="2" border size="small" class="exif">
          <el-descriptions-item :label="t('photos.takenAt')" :span="2">{{ formatDate(detail.taken_at, 'YYYY-MM-DD HH:mm:ss') }}</el-descriptions-item>
          <el-descriptions-item :label="t('photos.camera')">{{ detail.camera || '—' }}</el-descriptions-item>
          <el-descriptions-item :label="t('photos.lens')">{{ detail.lens || '—' }}</el-descriptions-item>
          <el-descriptions-item :label="t('photos.dimensions')">{{ detail.width }} × {{ detail.height }}</el-descriptions-item>
          <el-descriptions-item :label="t('photos.size')">{{ formatBytes(detail.size_bytes) }}</el-descriptions-item>
          <el-descriptions-item :label="t('files.modified')" :span="2">{{ formatDate(detail.created_at) }}</el-descriptions-item>
          <el-descriptions-item :label="t('photos.location')" :span="2">
            <template v-if="detail.gps_lat || detail.gps_lng">
              {{ detail.gps_lat.toFixed(5) }}, {{ detail.gps_lng.toFixed(5) }}
            </template>
            <span v-else class="pos-hint">{{ t('photos.noGps') }}</span>
          </el-descriptions-item>
        </el-descriptions>
      </div>
    </el-drawer>

    <!-- create album -->
    <el-dialog v-model="albumDialog" :title="t('photos.newAlbum')" width="430px">
      <el-form label-position="top">
        <el-form-item :label="t('photos.albumName')">
          <el-input v-model="albumForm.name" maxlength="60" />
        </el-form-item>
        <el-form-item :label="t('photos.albumDesc')">
          <el-input v-model="albumForm.description" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="albumDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="busy" @click="createAlbum">{{ t('common.create') }}</el-button>
      </template>
    </el-dialog>

    <!-- add to album -->
    <el-dialog v-model="addDialog" :title="t('photos.addToAlbum')" width="420px">
      <el-select v-model="addAlbumId" style="width: 100%" :placeholder="t('photos.albums')">
        <el-option v-for="a in albums" :key="a.id" :label="a.name" :value="a.id" />
      </el-select>
      <template #footer>
        <el-button @click="addDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :disabled="!addAlbumId" :loading="busy" @click="confirmAddToAlbum">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- share -->
    <el-dialog v-model="shareOpen" :title="t('files.shareTitle')" width="440px">
      <template v-if="!shareLink">
        <el-form label-position="top">
          <el-form-item :label="t('files.sharePassword')">
            <el-input v-model="shareForm.password" show-password />
          </el-form-item>
          <el-form-item :label="t('files.shareExpiry')">
            <el-input-number v-model="shareForm.hours" :min="0" :max="8760" style="width: 100%" />
          </el-form-item>
        </el-form>
      </template>
      <div v-else class="share-result">
        <el-result icon="success" :title="t('files.shareCreated')">
          <template #extra>
            <div class="share-link">
              <el-input :model-value="shareLink" readonly />
              <el-button type="primary" @click="copyShare">{{ t('common.copy') }}</el-button>
            </div>
          </template>
        </el-result>
      </div>
      <template #footer>
        <el-button @click="shareOpen = false">{{ t('common.close') }}</el-button>
        <el-button v-if="!shareLink" type="primary" :loading="busy" @click="submitShare">{{ t('files.createShare') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="uploadOpen" :title="t('photos.upload')" width="420px" :show-close="false" :close-on-click-modal="false">
      <el-progress :percentage="uploadPercent" :status="uploadPercent >= 100 ? 'success' : undefined" />
      <div class="pos-hint" style="margin-top: 8px">{{ uploadLabel }}</div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Aim, Camera, Refresh, FolderAdd, Folder, Delete, Download, Share, ArrowLeft, Location
} from '@element-plus/icons-vue'
import PageHeader from '../components/PageHeader.vue'
import EmptyState from '../components/EmptyState.vue'
import EChart from '../components/EChart.vue'
import { api, apiURL, errorMessage } from '../api'
import { useUiStore } from '../stores/ui'
import { formatBytes, formatDate } from '../utils/format'
import dayjs from 'dayjs'

const { t } = useI18n()
const ui = useUiStore()

const tab = ref('timeline')
const photos = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 60
const loading = ref(false)
const albums = ref([])
const albumView = ref(null)
const albumPhotos = ref([])
const timeline = ref([])
const filterYear = ref('')
const filterMonth = ref('')
const selectMode = ref(false)
const selectedIds = ref([])
const detailOpen = ref(false)
const detail = ref(null)
const photoInput = ref(null)
const albumDialog = ref(false)
const albumForm = reactive({ name: '', description: '' })
const addDialog = ref(false)
const addAlbumId = ref(null)
const addPhotoIds = ref([])
const shareOpen = ref(false)
const sharePhoto = ref(null)
const shareForm = reactive({ password: '', hours: 0 })
const shareLink = ref('')
const busy = ref(false)
const scanning = ref(false)
const uploadOpen = ref(false)
const uploadPercent = ref(0)
const uploadLabel = ref('')
const geoPhotos = ref([])

const years = computed(() => [...new Set((timeline.value || []).map((g) => g.year))].sort((a, b) => b - a))

const groupedPhotos = computed(() => {
  if (albumView.value) {
    return [{ key: 'album-' + albumView.value.id, label: albumView.value.name, items: albumPhotos.value }]
  }
  const map = new Map()
  for (const p of photos.value) {
    const key = dayjs(p.taken_at || p.created_at).format('YYYY-MM')
    if (!map.has(key)) map.set(key, [])
    map.get(key).push(p)
  }
  return [...map.entries()]
    .sort((a, b) => (a[0] < b[0] ? 1 : -1))
    .map(([key, items]) => ({ key, label: key.replace('-', ' · '), items }))
})

const mapOption = computed(() => {
  const text = ui.theme === 'dark' ? '#8ba1b4' : '#5c7186'
  const grid = ui.theme === 'dark' ? 'rgba(139,161,180,.12)' : 'rgba(92,113,134,.15)'
  return {
    backgroundColor: 'transparent',
    grid: { left: 56, right: 24, top: 24, bottom: 42 },
    tooltip: {
      formatter: (p) => {
        const item = geoPhotos.value[p.dataIndex]
        return item ? `${item.file_name}<br/>${formatDate(item.taken_at)}` : ''
      }
    },
    xAxis: {
      name: 'Longitude', nameTextStyle: { color: text },
      axisLabel: { color: text }, splitLine: { lineStyle: { color: grid } }, scale: true
    },
    yAxis: {
      name: 'Latitude', nameTextStyle: { color: text },
      axisLabel: { color: text }, splitLine: { lineStyle: { color: grid } }, scale: true
    },
    series: [{
      type: 'scatter',
      symbolSize: 14,
      itemStyle: { color: '#2dd4bf', shadowBlur: 12, shadowColor: 'rgba(45,212,191,.45)' },
      data: geoPhotos.value.map((p) => [p.gps_lng, p.gps_lat])
    }]
  }
})

function thumbURL(photo, size = 400) {
  return apiURL(`/photos/${photo.id}/thumb?size=${size}`)
}
function fileURL(photo) {
  return apiURL(`/photos/${photo.id}/file`)
}

function filterRange() {
  if (!filterYear.value) return {}
  const y = filterYear.value
  const start = filterMonth.value ? new Date(y, filterMonth.value - 1, 1) : new Date(y, 0, 1)
  const end = filterMonth.value ? new Date(y, filterMonth.value, 1) : new Date(y + 1, 0, 1)
  return { from: start.toISOString(), to: end.toISOString() }
}

async function loadPhotos(append = false) {
  if (albumView.value) return
  loading.value = true
  try {
    const payload = await api.photos.list({ page: page.value, page_size: pageSize, ...filterRange() })
    photos.value = append ? [...photos.value, ...payload.data] : payload.data
    total.value = payload.pagination.total
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function loadMore() {
  page.value += 1
  await loadPhotos(true)
}

function onFilterChange() {
  page.value = 1
  loadPhotos(false)
}

async function loadTimeline() {
  try {
    timeline.value = await api.photos.timeline()
  } catch (error) {
    timeline.value = []
  }
}

async function loadAlbums() {
  try {
    albums.value = await api.photos.albums()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

async function loadGeo() {
  try {
    geoPhotos.value = (await api.photos.map()) || []
  } catch (error) {
    geoPhotos.value = []
  }
}

async function loadAll() {
  page.value = 1
  await Promise.all([loadPhotos(false), loadTimeline(), loadAlbums()])
}

function pickPhotos() {
  photoInput.value?.click()
}

async function onPhotosPicked(event) {
  const files = Array.from(event.target.files || [])
  event.target.value = ''
  if (!files.length) return
  uploadOpen.value = true
  uploadPercent.value = 0
  uploadLabel.value = files.map((f) => f.name).join(', ')
  const form = new FormData()
  files.forEach((f) => form.append('files', f, f.name))
  try {
    await api.photos.upload(form, (evt) => {
      if (evt.total) uploadPercent.value = Math.round((evt.loaded / evt.total) * 100)
    })
    uploadPercent.value = 100
    ElMessage.success(t('photos.uploadSuccess'))
    setTimeout(() => { uploadOpen.value = false }, 500)
    await loadAll()
  } catch (error) {
    uploadOpen.value = false
    ElMessage.error(errorMessage(error))
  }
}

async function scan() {
  scanning.value = true
  try {
    const job = await api.photos.scan()
    ElMessage.success(t('photos.scanStarted'))
    await pollJob(job)
    await loadAll()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    scanning.value = false
  }
}

async function pollJob(job) {
  if (!job?.id) return
  for (let i = 0; i < 120; i += 1) {
    await new Promise((resolve) => setTimeout(resolve, 1000))
    try {
      const jobs = await api.jobs.list()
      const current = jobs.find((j) => j.id === job.id)
      if (current && current.status !== 'running') return current
    } catch (error) {
      return null
    }
  }
  return null
}

function toggleSelect() {
  selectMode.value = !selectMode.value
  if (!selectMode.value) selectedIds.value = []
}

function toggleSelectPhoto(id) {
  if (selectedIds.value.includes(id)) selectedIds.value = selectedIds.value.filter((x) => x !== id)
  else selectedIds.value = [...selectedIds.value, id]
}

function onPhotoClick(photo) {
  if (selectMode.value) {
    toggleSelectPhoto(photo.id)
    return
  }
  detail.value = photo
  detailOpen.value = true
}

function openShare(photo) {
  sharePhoto.value = photo
  shareForm.password = ''
  shareForm.hours = 0
  shareLink.value = ''
  shareOpen.value = true
}

async function submitShare() {
  busy.value = true
  try {
    const payload = await api.photos.share(sharePhoto.value.id, {
      password: shareForm.password || undefined,
      expires_hours: shareForm.hours
    })
    shareLink.value = `${window.location.origin}/s/${payload.share.token}`
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

async function copyShare() {
  try {
    await navigator.clipboard.writeText(shareLink.value)
    ElMessage.success(t('common.copied'))
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

function downloadPhoto(photo) {
  window.open(fileURL(photo), '_blank')
}

async function removePhoto(photo) {
  try {
    await ElMessageBox.confirm(t('photos.deleteConfirm'), t('common.delete'), {
      type: 'warning', confirmButtonText: t('common.delete'), cancelButtonText: t('common.cancel')
    })
  } catch (error) {
    return
  }
  try {
    await api.photos.remove(photo.id)
    ElMessage.success(t('photos.deleteSuccess'))
    detailOpen.value = false
    await loadAll()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

async function deleteSelected() {
  try {
    await ElMessageBox.confirm(t('photos.deleteConfirm'), t('common.delete'), {
      type: 'warning', confirmButtonText: t('common.delete'), cancelButtonText: t('common.cancel')
    })
  } catch (error) {
    return
  }
  try {
    for (const id of selectedIds.value) {
      await api.photos.remove(id)
    }
    ElMessage.success(t('photos.deleteSuccess'))
    selectedIds.value = []
    await loadAll()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

function openCreateAlbum() {
  albumForm.name = ''
  albumForm.description = ''
  albumDialog.value = true
}

async function createAlbum() {
  if (!albumForm.name.trim()) return
  busy.value = true
  try {
    await api.photos.createAlbum({ name: albumForm.name.trim(), description: albumForm.description })
    ElMessage.success(t('photos.albumCreated'))
    albumDialog.value = false
    await loadAlbums()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

async function openAlbum(album) {
  try {
    const payload = await api.photos.album(album.id)
    albumView.value = payload.album
    albumPhotos.value = payload.photos || []
    tab.value = 'timeline'
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

function closeAlbum() {
  albumView.value = null
  albumPhotos.value = []
  loadPhotos(false)
}

async function removeAlbum(album) {
  try {
    await ElMessageBox.confirm(t('photos.deleteAlbumConfirm'), t('common.delete'), {
      type: 'warning', confirmButtonText: t('common.delete'), cancelButtonText: t('common.cancel')
    })
  } catch (error) {
    return
  }
  try {
    await api.photos.deleteAlbum(album.id)
    await loadAlbums()
    if (albumView.value?.id === album.id) closeAlbum()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

async function deleteAlbum() {
  if (albumView.value) await removeAlbum(albumView.value)
}

function openAddToAlbum(photo) {
  addPhotoIds.value = photo ? [photo.id] : [...selectedIds.value]
  addAlbumId.value = null
  addDialog.value = true
}

async function confirmAddToAlbum() {
  busy.value = true
  try {
    await api.photos.addToAlbum(addAlbumId.value, addPhotoIds.value)
    ElMessage.success(t('common.operationSuccess'))
    addDialog.value = false
    addPhotoIds.value = []
    await loadAlbums()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

onMounted(() => {
  loadAll()
  loadGeo()
})
onBeforeUnmount(() => {})
</script>

<style scoped>
.filter-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 0 4px;
}
.tabs {
  min-width: 260px;
}
.tabs :deep(.el-tabs__header) { margin: 0; }
.tabs :deep(.el-tabs__nav-wrap::after) { display: none; }
.select-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  font-size: 12.5px;
}
.album-head {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
}
.timeline { display: flex; flex-direction: column; gap: 20px; }
.month-head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 10px;
  padding-left: 2px;
}
.month {
  font-size: 15px;
  font-weight: 600;
  letter-spacing: 0.4px;
}
.photo-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(168px, 1fr));
  gap: 10px;
}
.photo-cell {
  position: relative;
  aspect-ratio: 4 / 3;
  border-radius: 12px;
  overflow: hidden;
  border: 1px solid var(--pos-border);
  background: var(--pos-panel-2);
  padding: 0;
  cursor: zoom-in;
  transition: transform 0.18s ease, border-color 0.18s ease;
}
.photo-cell:hover { transform: translateY(-2px); border-color: rgba(45, 212, 191, 0.45); }
.photo-cell.selected { border-color: var(--pos-accent); box-shadow: 0 0 0 2px var(--pos-accent-soft); }
.photo-cell img { width: 100%; height: 100%; object-fit: cover; display: block; }
.photo-overlay {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  padding: 18px 10px 8px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  color: #e6edf3;
  font-size: 10.5px;
  background: linear-gradient(transparent, rgba(0, 0, 0, 0.62));
  opacity: 0;
  transition: opacity 0.18s ease;
}
.photo-cell:hover .photo-overlay { opacity: 1; }
.load-more { display: flex; justify-content: center; padding: 8px 0 24px; }
.album-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 14px;
}
.album-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  cursor: pointer;
  text-align: left;
  color: var(--pos-text);
}
.album-cover {
  width: 74px;
  height: 74px;
  border-radius: 11px;
  overflow: hidden;
  background: var(--pos-panel-2);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--pos-accent);
  flex-shrink: 0;
}
.album-cover img { width: 100%; height: 100%; object-fit: cover; }
.album-meta { display: flex; flex-direction: column; gap: 4px; flex: 1; min-width: 0; }
.album-meta strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.map-panel { padding: 18px 20px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px; }
.geo-list { display: flex; flex-direction: column; gap: 6px; margin-top: 12px; max-height: 220px; overflow: auto; }
.geo-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 8px;
  border-radius: 8px;
  font-size: 12px;
  background: var(--pos-panel-2);
}
.geo-row span:nth-child(2) { flex: 1; }
.detail { display: flex; flex-direction: column; gap: 14px; }
.detail-image { width: 100%; max-height: 340px; border-radius: 12px; background: var(--pos-panel-2); }
.detail-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.exif { margin-top: 4px; }
.share-link { display: flex; gap: 8px; }
@media (max-width: 640px) {
  .photo-grid { grid-template-columns: repeat(auto-fill, minmax(130px, 1fr)); }
}
</style>
