<template>
  <div class="pos-page photos-page" :style="{ '--photos-sidebar-width': layout.sidebars.photos.visible ? `${layout.sidebars.photos.width}px` : '0px' }">
    <PageHeader :title="t('photos.title')" :subtitle="t('photos.subtitle')">
      <el-button :icon="Aim" :loading="selectMode" :type="selectMode ? 'primary' : 'default'" @click="toggleSelect">
        {{ t('photos.selectPhotos') }}
      </el-button>
      <el-button :icon="Camera" @click="pickPhotos">{{ t('photos.upload') }}</el-button>
      <el-button :icon="FolderAdd" @click="openSources">{{ t('photos.addSource') }}</el-button>
      <el-button :icon="View" @click="toggleAspect">{{ aspect === 'cover' ? t('photos.originalRatio') : t('photos.fixedRatio') }}</el-button>
      <el-button :icon="Operation" @click="layout.toggle('photos')">{{ t('photos.sidebar') }}</el-button>
      <el-button :icon="Refresh" :loading="scanning" @click="scan">{{ t('photos.scan') }}</el-button>
      <el-button type="primary" :icon="FolderAdd" @click="openCreateAlbum">{{ t('photos.newAlbum') }}</el-button>
    </PageHeader>

    <div class="filter-bar">
      <span class="section-title">{{ sectionLabel }}</span>
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
    <template v-if="tab === 'timeline' && showTimeline">
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
          <div class="photo-grid" :class="{ original: aspect === 'original' }" @pointerdown="startPhotoBand">
            <div
              v-for="photo in group.items"
              :key="photo.id"
              class="photo-cell"
              role="button"
              tabindex="0"
              :data-id="photo.id"
              :class="{ selected: selectedIds.includes(photo.id) }"
              @click="onPhotoClick(photo)"
              @keyup.enter="onPhotoClick(photo)"
            >
              <img :src="thumbURL(photo, 400)" :alt="photo.file_name" loading="lazy" draggable="false" />
              <span class="photo-overlay">
                <span>{{ formatDate(photo.taken_at, 'MM-DD HH:mm') }}</span>
                <span class="cell-actions">
                  <el-button v-if="section === 'deleted'" link :icon="RefreshLeft" class="cell-btn" @click.stop="restorePhoto(photo)" />
                  <el-button v-else link :icon="photo.favorite ? StarFilled : Star" class="cell-btn" @click.stop="toggleFavorite(photo)" />
                  <el-checkbox v-if="selectMode" :model-value="selectedIds.includes(photo.id)" @click.stop @change="toggleSelectPhoto(photo.id)" />
                </span>
              </span>
            </div>
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
    <template v-else-if="tab === 'map'">
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

    <!-- dates -->
    <template v-if="section === 'dates'">
      <div class="date-panel pos-panel">
        <div class="date-head">
          <el-radio-group v-model="dateMode" size="small">
            <el-radio-button value="day">{{ t('photos.perDay') }}</el-radio-button>
            <el-radio-button value="month">{{ t('photos.perMonth') }}</el-radio-button>
            <el-radio-button value="year">{{ t('photos.perYear') }}</el-radio-button>
          </el-radio-group>
          <span class="pos-hint">{{ t('photos.clickDateHint') }}</span>
        </div>
        <div class="date-groups">
          <button v-for="bucket in dateBuckets" :key="bucket.key" type="button" class="date-item" @click="pickDate(bucket)">
            <span>{{ bucket.label }}</span><span class="pos-hint">{{ t('photos.photoCount', { n: bucket.count }) }}</span>
          </button>
        </div>
      </div>
    </template>

    <!-- duplicates -->
    <template v-if="section === 'duplicates'">
      <div class="duplicate-head pos-panel">
        <div>
          <strong>{{ t('photos.duplicates') }}</strong>
          <p class="pos-hint">{{ t('photos.duplicatesHint') }}</p>
        </div>
        <el-button type="primary" :loading="busy" :disabled="!duplicateGroups.length" @click="mergeDuplicates">{{ t('photos.mergeDuplicates') }}</el-button>
      </div>
      <div v-if="!duplicateGroups.length" class="pos-panel empty-block">{{ t('photos.noDuplicates') }}</div>
      <div v-for="group in duplicateGroups" :key="group.sha256" class="duplicate-group pos-panel">
        <div class="duplicate-grid">
          <img v-for="photo in group.photos" :key="photo.id" :src="thumbURL(photo, 300)" alt="" @click="openDetail(photo)" />
        </div>
        <span class="pos-hint">{{ group.photos.length }} {{ t('photos.copies') }} · {{ t('photos.keepBest') }}</span>
      </div>
    </template>

    <!-- memories / people placeholders -->
    <template v-if="section === 'memories'">
      <div class="pos-panel empty-block">
        <el-icon :size="28"><Clock /></el-icon>
        <strong>{{ t('photos.memories') }}</strong>
        <p class="pos-hint">{{ t('photos.memoriesHint') }}</p>
      </div>
    </template>
    <template v-if="section === 'people'">
      <div class="pos-panel empty-block">
        <el-icon :size="28"><User /></el-icon>
        <strong>{{ t('photos.people') }}</strong>
        <p class="pos-hint">{{ t('photos.peopleHint') }}</p>
      </div>
    </template>

    <input ref="photoInput" type="file" accept="image/*" multiple hidden @change="onPhotosPicked" />

    <!-- photo detail drawer -->
    <el-drawer v-model="detailOpen" :title="detail?.file_name || t('photos.viewPhoto')" size="480px">
      <div v-if="detail" class="detail">
        <el-image :src="fileURL(detail)" fit="contain" class="detail-image" :preview-src-list="[fileURL(detail)]" preview-teleported />
        <div class="detail-actions">
          <el-button :icon="detail.favorite ? StarFilled : Star" @click="toggleFavorite(detail)">{{ t('photos.favorite') }}</el-button>
          <el-button v-if="detail.deleted_at" :icon="RefreshLeft" @click="restorePhoto(detail)">{{ t('photos.restore') }}</el-button>
          <el-button :icon="Download" @click="downloadPhoto(detail)">{{ t('photos.downloadOriginal') }}</el-button>
          <el-button :icon="Share" @click="openShare(detail)">{{ t('photos.sharePhoto') }}</el-button>
          <el-button :icon="FolderAdd" @click="openAddToAlbum(detail)">{{ t('photos.addToAlbum') }}</el-button>
          <el-button v-if="detail.deleted_at" type="danger" plain :icon="Delete" @click="purgePhoto(detail)">{{ t('photos.purge') }}</el-button>
          <el-button v-else type="danger" plain :icon="Delete" @click="removePhoto(detail)">{{ t('common.delete') }}</el-button>
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


    <el-dialog v-model="sourceDialog" :title="t('photos.addSource')" width="620px">
      <div class="source-layout">
        <div class="source-browser">
          <div class="source-path">
            <el-button link :icon="ArrowLeft" :disabled="!sourcePath" @click="sourceUp">{{ t('common.back') }}</el-button>
            <span class="pos-mono">/{{ sourcePath }}</span>
          </div>
          <div class="source-list">
            <div v-for="entry in sourceEntries" :key="entry.path" class="source-row">
              <el-checkbox :model-value="sourceSelected.includes(entry.path)" @change="toggleSource(entry)" />
              <button type="button" class="source-name" @click="browseSource(entry.path)">{{ entry.name }}</button>
            </div>
            <div v-if="!sourceEntries.length" class="pos-hint" style="padding: 16px">{{ t('common.empty') }}</div>
          </div>
        </div>
        <div class="source-selected">
          <strong>{{ t('photos.selectedFolders') }}</strong>
          <div v-for="path in sourceSelected" :key="path" class="selected-row">
            <span>{{ path }}</span>
            <button type="button" @click="sourceSelected = sourceSelected.filter((p) => p !== path)"><el-icon><Close /></el-icon></button>
          </div>
          <div v-if="!sourceSelected.length" class="pos-hint">{{ t('photos.selectFoldersHint') }}</div>
          <el-button type="primary" :loading="busy" :disabled="!sourceSelected.length" @click="saveSources">{{ t('photos.addSelected') }}</el-button>
          <div class="source-existing">
            <strong>{{ t('photos.sources') }}</strong>
            <div v-for="src in sources" :key="src.id" class="selected-row">
              <span>{{ src.path }}</span>
              <el-button link type="danger" :icon="Delete" @click="removeSource(src)" />
            </div>
          </div>
        </div>
      </div>
    </el-dialog>
    <el-dialog v-model="uploadOpen" :title="t('photos.upload')" width="420px" :show-close="false" :close-on-click-modal="false">
      <el-progress :percentage="uploadPercent" :status="uploadPercent >= 100 ? 'success' : undefined" />
      <div class="pos-hint" style="margin-top: 8px">{{ uploadLabel }}</div>
    </el-dialog>

    <div v-if="photoBand.active" class="photo-band" :style="{ left: `${photoBand.x}px`, top: `${photoBand.y}px`, width: `${photoBand.w}px`, height: `${photoBand.h}px` }" />

    <Teleport to="body">
    <transition name="lightbox">
      <div v-if="lightboxOpen" class="lightbox" @click.self="lightboxOpen = false">
        <button type="button" class="lightbox-close" @click="lightboxOpen = false"><el-icon :size="22"><Close /></el-icon></button>
        <button v-if="lightboxPhotos.length > 1" type="button" class="lightbox-nav prev" @click.stop="stepLightbox(-1)"><el-icon><ArrowLeft /></el-icon></button>
        <img :src="fileURL(lightboxPhoto)" :alt="lightboxPhoto?.file_name" class="lightbox-image" />
        <button v-if="lightboxPhotos.length > 1" type="button" class="lightbox-nav next" @click.stop="stepLightbox(1)"><el-icon><ArrowRight /></el-icon></button>
        <div class="lightbox-bar" @click.stop>
          <span>{{ lightboxPhoto?.file_name }}</span>
          <span class="pos-spacer" />
          <el-button link :icon="lightboxPhoto?.favorite ? StarFilled : Star" @click="toggleFavorite(lightboxPhoto)">{{ t('photos.favorite') }}</el-button>
          <el-button link :icon="InfoFilled" @click="openDetail(lightboxPhoto)">{{ t('common.details') }}</el-button>
          <el-button link :icon="Download" @click="downloadPhoto(lightboxPhoto)">{{ t('common.download') }}</el-button>
        </div>
      </div>
    </transition>
    </Teleport>

    <aside v-show="layout.sidebars.photos.visible" class="photos-sidebar" :style="{ width: `${layout.sidebars.photos.width}px` }">
      <SidebarResizer
        side="right"
        :width="layout.sidebars.photos.width"
        @resize="(w) => layout.setWidth('photos', w)"
      />
      <div class="sidebar-head">
        <strong>{{ t('photos.library') }}</strong>
        <button type="button" class="sidebar-close" @click="layout.toggle('photos')"><el-icon><Close /></el-icon></button>
      </div>
      <nav class="photo-nav">
        <button
          v-for="(item, index) in orderedSections"
          :key="item.key"
          type="button"
          class="photo-nav-item"
          :class="{ active: section === item.key }"
          draggable="true"
          @click="selectSection(item.key)"
          @dragstart="dragIndex = index"
          @dragover.prevent
          @drop="dropSection(index)"
        >
          <el-icon :size="15"><component :is="item.icon" /></el-icon>
          <span>{{ t(item.label) }}</span>
        </button>
      </nav>
      <div v-if="section === 'dates'" class="sidebar-dates">
        <el-radio-group v-model="dateMode" size="small" style="margin-bottom: 8px">
          <el-radio-button value="day">{{ t('photos.perDay') }}</el-radio-button>
          <el-radio-button value="month">{{ t('photos.perMonth') }}</el-radio-button>
          <el-radio-button value="year">{{ t('photos.perYear') }}</el-radio-button>
        </el-radio-group>
      </div>
      <div class="sidebar-foot">
        <span class="pos-hint">{{ t('photos.dragHint') }}</span>
      </div>
    </aside>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Aim, ArrowLeft, ArrowRight, Camera, Clock, Close, Delete, Download, FolderAdd, Folder,
  InfoFilled, Location, Operation, Refresh, RefreshLeft, Share, Star, StarFilled, User, View
} from '@element-plus/icons-vue'
import PageHeader from '../components/PageHeader.vue'
import EmptyState from '../components/EmptyState.vue'
import SidebarResizer from '../components/SidebarResizer.vue'
import EChart from '../components/EChart.vue'
import { api, apiURL, errorMessage } from '../api'
import { useUiStore } from '../stores/ui'
import { useLayoutStore } from '../stores/layout'
import { formatBytes, formatDate } from '../utils/format'
import dayjs from 'dayjs'

const { t } = useI18n()
const ui = useUiStore()
const layout = useLayoutStore()

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
const section = ref('library')
const aspect = ref(localStorage.getItem('pos_photo_aspect') || 'cover')
const filterDate = ref('')
const dateMode = ref('day')
const lightboxOpen = ref(false)
const lightboxIndex = ref(0)
const duplicateGroups = ref([])
const sourceDialog = ref(false)
const sourcePath = ref('')
const sourceEntries = ref([])
const sourceSelected = ref([])
const sources = ref([])
const dragIndex = ref(-1)
const photoBand = reactive({ active: false, x: 0, y: 0, w: 0, h: 0, startX: 0, startY: 0 })
const suppressPhotoClick = ref(false)

const sectionDefs = [
  { key: 'library', label: 'photos.library', icon: 'Picture' },
  { key: 'recent', label: 'photos.recent', icon: 'Clock' },
  { key: 'favorites', label: 'photos.favorites', icon: 'Star' },
  { key: 'deleted', label: 'photos.deleted', icon: 'Delete' },
  { key: 'dates', label: 'photos.dates', icon: 'Calendar' },
  { key: 'memories', label: 'photos.memories', icon: 'Sunny' },
  { key: 'people', label: 'photos.people', icon: 'User' },
  { key: 'duplicates', label: 'photos.duplicates', icon: 'CopyDocument' },
  { key: 'albums', label: 'photos.albums', icon: 'Folder' },
  { key: 'map', label: 'photos.map', icon: 'Location' }
]
const sectionOrder = ref(JSON.parse(localStorage.getItem('pos_photo_sections') || '[]'))
if (!Array.isArray(sectionOrder.value) || sectionOrder.value.length !== sectionDefs.length) {
  sectionOrder.value = sectionDefs.map((s) => s.key)
}
const orderedSections = computed(() => sectionOrder.value.map((key) => sectionDefs.find((s) => s.key === key)).filter(Boolean))
const sectionLabel = computed(() => {
  const def = sectionDefs.find((s) => s.key === section.value)
  return def ? t(def.label) : ''
})
const showTimeline = computed(() => ['library', 'recent', 'favorites', 'deleted'].includes(section.value))
const lightboxPhotos = computed(() => albumView.value ? albumPhotos.value : photos.value)
const lightboxPhoto = computed(() => lightboxPhotos.value[lightboxIndex.value] || null)
const dateBuckets = computed(() => {
  if (dateMode.value === 'year') {
    return (timeline.value || []).map((y) => ({ key: `y-${y.year}`, label: String(y.year), count: y.count, year: y.year }))
  }
  if (dateMode.value === 'month') {
    const out = []
    for (const y of timeline.value || []) {
      for (const m of y.months || []) out.push({ key: `m-${y.year}-${m.month}`, label: `${y.year}-${String(m.month).padStart(2, '0')}`, count: m.count, year: y.year, month: m.month })
    }
    return out
  }
  const out = []
  for (const y of timeline.value || []) {
    for (const m of y.months || []) {
      for (const d of m.days || []) out.push({ key: `d-${d.date}`, label: d.date, count: d.count, date: d.date })
    }
  }
  return out
})

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
  if (filterDate.value) {
    const start = new Date(`${filterDate.value}T00:00:00`)
    const end = new Date(start.getTime() + 86400000)
    return { from: start.toISOString(), to: end.toISOString() }
  }
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
    const view = section.value === 'recent' ? 'recent' : section.value === 'favorites' ? 'favorites' : section.value === 'deleted' ? 'deleted' : ''
    const payload = await api.photos.list({ page: page.value, page_size: pageSize, view, ...filterRange() })
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
  if (suppressPhotoClick.value) {
    suppressPhotoClick.value = false
    return
  }
  if (selectMode.value) {
    toggleSelectPhoto(photo.id)
    return
  }
  openLightbox(photo)
}

function startPhotoBand(event) {
  if (event.button !== 0) return
  const startX = event.clientX
  const startY = event.clientY
  photoBand.active = false
  photoBand.startX = startX
  photoBand.startY = startY
  let moved = false
  const onMove = (e) => {
    if (!moved && Math.abs(e.clientX - startX) + Math.abs(e.clientY - startY) < 6) return
    moved = true
    photoBand.active = true
    photoBand.x = Math.min(startX, e.clientX)
    photoBand.y = Math.min(startY, e.clientY)
    photoBand.w = Math.abs(e.clientX - startX)
    photoBand.h = Math.abs(e.clientY - startY)
  }
  const onUp = () => {
    window.removeEventListener('pointermove', onMove)
    window.removeEventListener('pointerup', onUp)
    if (moved) {
      const picked = []
      document.querySelectorAll('.photo-cell').forEach((el) => {
        const r = el.getBoundingClientRect()
        if (r.left < photoBand.x + photoBand.w && r.right > photoBand.x && r.top < photoBand.y + photoBand.h && r.bottom > photoBand.y) {
          const id = Number(el.dataset.id)
          if (id) picked.push(id)
        }
      })
      selectedIds.value = picked
      selectMode.value = picked.length > 0
      suppressPhotoClick.value = true
      setTimeout(() => { suppressPhotoClick.value = false }, 60)
    }
    photoBand.active = false
  }
  window.addEventListener('pointermove', onMove)
  window.addEventListener('pointerup', onUp)
}

function openLightbox(photo) {
  const index = lightboxPhotos.value.findIndex((p) => p.id === photo.id)
  lightboxIndex.value = Math.max(0, index)
  lightboxOpen.value = true
}
function stepLightbox(delta) {
  if (!lightboxPhotos.value.length) return
  lightboxIndex.value = (lightboxIndex.value + delta + lightboxPhotos.value.length) % lightboxPhotos.value.length
}
function openDetail(photo) {
  detail.value = photo
  detailOpen.value = true
}
function toggleAspect() {
  aspect.value = aspect.value === 'cover' ? 'original' : 'cover'
  localStorage.setItem('pos_photo_aspect', aspect.value)
}
async function toggleFavorite(photo) {
  if (!photo) return
  try {
    await api.photos.favorite(photo.id, !photo.favorite)
    photo.favorite = !photo.favorite
    if (detail.value?.id === photo.id) detail.value = { ...photo }
    ElMessage.success(photo.favorite ? t('photos.favorited') : t('photos.unfavorited'))
  } catch (error) { ElMessage.error(errorMessage(error)) }
}
async function purgePhoto(photo) {
  try {
    await ElMessageBox.confirm(t('photos.purgeConfirm'), t('common.delete'), {
      type: 'warning', confirmButtonText: t('common.delete'), cancelButtonText: t('common.cancel')
    })
  } catch (error) { return }
  try {
    await api.photos.purge(photo.id)
    detailOpen.value = false
    lightboxOpen.value = false
    ElMessage.success(t('photos.purged'))
    await loadAll()
  } catch (error) { ElMessage.error(errorMessage(error)) }
}
async function restorePhoto(photo) {
  try {
    await api.photos.restore(photo.id)
    ElMessage.success(t('photos.restored'))
    lightboxOpen.value = false
    await loadAll()
  } catch (error) { ElMessage.error(errorMessage(error)) }
}
function selectSection(key) {
  section.value = key
  filterDate.value = ''
  filterYear.value = ''
  filterMonth.value = ''
  page.value = 1
  if (key === 'albums') { tab.value = 'albums'; loadAlbums(); return }
  if (key === 'map') { tab.value = 'map'; loadGeo(); return }
  tab.value = 'timeline'
  if (key === 'duplicates') { loadDuplicates(); return }
  if (key === 'dates' || key === 'memories' || key === 'people') return
  loadPhotos(false)
}
function pickDate(bucket) {
  if (bucket.date) filterDate.value = bucket.date
  if (bucket.year) filterYear.value = bucket.year
  if (bucket.month) filterMonth.value = bucket.month
  section.value = 'library'
  tab.value = 'timeline'
  page.value = 1
  loadPhotos(false)
}
function dropSection(target) {
  if (dragIndex.value < 0 || dragIndex.value === target) return
  const copy = [...sectionOrder.value]
  const [moved] = copy.splice(dragIndex.value, 1)
  copy.splice(target, 0, moved)
  sectionOrder.value = copy
  localStorage.setItem('pos_photo_sections', JSON.stringify(copy))
  dragIndex.value = -1
}
async function loadDuplicates() {
  try { duplicateGroups.value = await api.photos.duplicates() || [] } catch (e) { duplicateGroups.value = [] }
}
async function mergeDuplicates() {
  try {
    const result = await api.photos.mergeDuplicates()
    ElMessage.success(t('photos.duplicatesMerged', { n: result.removed || 0 }))
    await Promise.all([loadDuplicates(), loadAll()])
  } catch (error) { ElMessage.error(errorMessage(error)) }
}
async function loadSources() {
  try { sources.value = await api.photos.sources() || [] } catch (e) { sources.value = [] }
}
async function openSources() {
  sourceSelected.value = []
  await Promise.all([loadSources(), browseSource('')])
  sourceDialog.value = true
}
async function browseSource(path) {
  sourcePath.value = path || ''
  try {
    const payload = await api.files.list({ path: sourcePath.value })
    sourceEntries.value = (payload.listing?.entries || []).filter((e) => e.is_dir)
  } catch (error) { sourceEntries.value = []; ElMessage.error(errorMessage(error)) }
}
function sourceUp() {
  const parts = sourcePath.value.split('/').filter(Boolean)
  parts.pop()
  browseSource(parts.join('/'))
}
function toggleSource(entry) {
  if (sourceSelected.value.includes(entry.path)) sourceSelected.value = sourceSelected.value.filter((p) => p !== entry.path)
  else sourceSelected.value = [...sourceSelected.value, entry.path]
}
async function saveSources() {
  busy.value = true
  try {
    await api.photos.addSources(sourceSelected.value)
    ElMessage.success(t('photos.sourceAdded'))
    sourceSelected.value = []
    await Promise.all([loadSources(), loadAll()])
  } catch (error) { ElMessage.error(errorMessage(error)) }
  finally { busy.value = false }
}
async function removeSource(src) {
  try {
    await api.photos.removeSource(src.id)
    await loadSources()
  } catch (error) { ElMessage.error(errorMessage(error)) }
}
function onLightboxKey(event) {
  if (!lightboxOpen.value) return
  if (event.key === 'Escape') lightboxOpen.value = false
  if (event.key === 'ArrowLeft') stepLightbox(-1)
  if (event.key === 'ArrowRight') stepLightbox(1)
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
    lightboxOpen.value = false
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
  loadSources()
  window.addEventListener('keydown', onLightboxKey)
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onLightboxKey)
})
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

.photos-page { position: relative; padding-right: calc(var(--photos-sidebar-width, 0px) + 14px); }
.photos-sidebar {
  position: absolute;
  top: 0;
  right: 0;
  width: var(--photos-sidebar-width, 232px);
  min-height: 560px;
  max-height: calc(100vh - 150px);
  display: flex;
  flex-direction: column;
  background: var(--pos-panel);
  border: 1px solid var(--pos-border);
  padding: 6px;
  z-index: 4;
}
.sidebar-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px;
  border-bottom: 1px solid var(--pos-border);
}
.sidebar-head strong { font-size: 13px; }
.sidebar-close { border: none; background: transparent; color: var(--pos-muted); cursor: pointer; }
.photo-nav { display: flex; flex-direction: column; gap: 1px; padding: 5px 0; overflow: auto; }
.photo-nav-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 9px;
  border: none;
  border-left: 2px solid transparent;
  background: transparent;
  color: var(--pos-text-2);
  font-size: 12px;
  text-align: left;
  cursor: pointer;
}
.photo-nav-item:hover { background: var(--pos-panel-2); }
.photo-nav-item.active { color: var(--pos-accent); background: var(--pos-accent-soft); border-left-color: var(--pos-accent); font-weight: 600; }
.sidebar-dates { padding: 8px; border-top: 1px solid var(--pos-border); }
.sidebar-foot { margin-top: auto; padding: 8px; border-top: 1px solid var(--pos-border); }
.section-title { font-size: 14px; font-weight: 650; }
.filter-bar .tabs, .filter-bar .el-select { display: none; }
.photo-band { position: fixed; z-index: 8800; border: 1px solid var(--pos-accent); background: color-mix(in srgb, var(--pos-accent) 12%, transparent); pointer-events: none; }
.photo-grid { gap: 0; user-select: none; grid-template-columns: repeat(auto-fill, minmax(190px, 1fr)); }
.photo-cell {
  aspect-ratio: 1;
  border-radius: 0;
  border-color: transparent;
  transition: none;
}
.photo-cell:hover { transform: none; border-color: transparent; }
.photo-grid.original { display: block; columns: 4 190px; column-gap: 0; }
.photo-grid.original .photo-cell { display: block; width: 100%; aspect-ratio: auto; margin: 0; break-inside: avoid; }
.photo-grid.original .photo-cell img { width: 100%; height: auto; }
.cell-actions { display: flex; align-items: center; gap: 4px; }
.cell-btn { color: #fff !important; padding: 0 !important; height: 20px !important; }
.date-panel { padding: 14px; }
.date-head { display: flex; align-items: center; gap: 12px; margin-bottom: 10px; flex-wrap: wrap; }
.date-groups { display: flex; flex-direction: column; gap: 1px; }
.date-item { display: flex; justify-content: space-between; padding: 9px 10px; border: none; border-bottom: 1px solid var(--pos-border); background: transparent; color: var(--pos-text); cursor: pointer; font-size: 12px; }
.date-item:hover { background: var(--pos-panel-2); }
.duplicate-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 14px; }
.duplicate-head p { margin: 3px 0 0; }
.duplicate-group { padding: 10px; }
.duplicate-grid { display: flex; gap: 0; flex-wrap: wrap; }
.duplicate-grid img { width: 110px; height: 110px; object-fit: cover; cursor: pointer; }
.empty-block { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 8px; padding: 40px 20px; text-align: center; color: var(--pos-muted); min-height: 220px; }
.empty-block strong { color: var(--pos-text); font-size: 14px; }
.lightbox { position: fixed; inset: 0; z-index: 9000; background: rgba(5, 9, 13, 0.95); display: flex; align-items: center; justify-content: center; }
.lightbox-image { max-width: calc(100vw - 90px); max-height: calc(100vh - 100px); object-fit: contain; border-radius: 0; }
.lightbox-close { position: absolute; right: 18px; top: 16px; border: none; background: transparent; color: #fff; cursor: pointer; }
.lightbox-nav { position: absolute; top: 50%; transform: translateY(-50%); width: 48px; height: 64px; border: 1px solid rgba(255,255,255,.2); background: rgba(0,0,0,.3); color: #fff; cursor: pointer; }
.lightbox-nav.prev { left: 14px; }
.lightbox-nav.next { right: 14px; }
.lightbox-bar { position: absolute; left: 0; right: 0; bottom: 0; height: 52px; display: flex; align-items: center; gap: 8px; padding: 0 18px; color: #fff; background: rgba(0,0,0,.45); font-size: 12px; }
.lightbox-bar .el-button { color: #fff; }
.lightbox-enter-active, .lightbox-leave-active { transition: opacity .15s ease; }
.lightbox-enter-from, .lightbox-leave-to { opacity: 0; }
.source-layout { display: grid; grid-template-columns: minmax(0, 1fr) 220px; gap: 14px; }
.source-browser { border: 1px solid var(--pos-border); min-width: 0; }
.source-path { display: flex; align-items: center; gap: 8px; padding: 6px 8px; border-bottom: 1px solid var(--pos-border); }
.source-list { max-height: 320px; overflow: auto; }
.source-row { display: flex; align-items: center; gap: 6px; padding: 5px 8px; }
.source-name { border: none; background: transparent; color: var(--pos-text); cursor: pointer; text-align: left; padding: 4px; flex: 1; }
.source-name:hover { color: var(--pos-accent); }
.source-selected { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
.selected-row { display: flex; align-items: center; justify-content: space-between; gap: 6px; padding: 4px 6px; background: var(--pos-panel-2); font-size: 11px; min-width: 0; }
.selected-row span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.selected-row button { border: none; background: transparent; color: var(--pos-muted); cursor: pointer; }
.source-existing { display: flex; flex-direction: column; gap: 5px; border-top: 1px solid var(--pos-border); padding-top: 8px; }
.detail-image { border-radius: 0; }
@media (max-width: 900px) {
  .photos-page { padding-right: 0; }
  .photos-sidebar { position: static; width: 100% !important; max-height: none; margin-top: 10px; }
  .photo-grid { grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); }
  .photo-grid.original { columns: 2 140px; }
  .source-layout { grid-template-columns: 1fr; }
}
</style>
