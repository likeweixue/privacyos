<template>
  <div class="pos-page">
    <PageHeader :title="t('movies.title')" :subtitle="t('movies.subtitle')">
      <el-button :icon="Refresh" :loading="loading" @click="reload">{{ t('common.refresh') }}</el-button>
      <el-button :icon="Setting" v-if="auth.isAdmin" @click="openTmdb">{{ t('movies.tmdbSettings') }}</el-button>
      <el-button :icon="RefreshRight" :loading="scanning" @click="scan">{{ t('movies.scan') }}</el-button>
      <el-button type="primary" :icon="MagicStick" :loading="scraping" @click="scrapeAll">{{ t('movies.scrape') }}</el-button>
    </PageHeader>

    <div class="filters pos-panel">
      <el-input
        v-model="filters.q"
        :placeholder="t('common.searchPlaceholder')"
        clearable
        :prefix-icon="Search"
        class="search"
        @keyup.enter="applyFilters"
      />
      <el-select v-model="filters.kind" clearable :placeholder="t('movies.allKinds')" style="width: 150px" @change="applyFilters">
        <el-option :label="t('movies.movie')" value="movie" />
        <el-option :label="t('movies.series')" value="series" />
        <el-option :label="t('movies.anime')" value="anime" />
        <el-option :label="t('movies.documentary')" value="documentary" />
      </el-select>
      <el-select v-model="filters.year" clearable :placeholder="t('movies.year')" style="width: 118px" @change="applyFilters">
        <el-option v-for="y in meta.years || []" :key="y" :label="String(y)" :value="y" />
      </el-select>
      <el-select v-model="filters.sort" :placeholder="t('movies.sort')" style="width: 150px" @change="applyFilters">
        <el-option :label="t('movies.sortTitle')" value="title" />
        <el-option :label="t('movies.sortNewest')" value="newest" />
        <el-option :label="t('movies.sortRating')" value="rating" />
        <el-option :label="t('movies.sortRecent')" value="recent" />
      </el-select>
      <el-button :icon="Search" type="primary" plain @click="applyFilters">{{ t('common.search') }}</el-button>
      <div class="pos-spacer" />
      <span class="pos-hint">{{ t('common.total') }} {{ total }}</span>
    </div>

    <div v-if="!loading && !movies.length" class="pos-panel">
      <EmptyState :title="t('movies.empty')" :description="t('movies.emptyHint')" icon="VideoCamera">
        <el-button type="primary" :icon="RefreshRight" @click="scan">{{ t('movies.scan') }}</el-button>
        <el-button v-if="auth.isAdmin" :icon="Setting" @click="openTmdb">{{ t('movies.tmdbSettings') }}</el-button>
      </EmptyState>
    </div>

    <div v-else v-loading="loading" class="poster-grid">
      <button v-for="movie in movies" :key="movie.id" type="button" class="poster-card" @click="openDetail(movie)">
        <div class="poster">
          <img v-if="movie.poster_url" :src="posterURL(movie, 400)" :alt="movie.title" loading="lazy" />
          <div v-else class="poster-fallback">
            <span>{{ (movie.title || '?').slice(0, 1) }}</span>
          </div>
          <span class="kind-tag">{{ kindLabel(movie.kind) }}</span>
          <span v-if="movie.rating" class="rating"><el-icon><StarFilled /></el-icon>{{ movie.rating.toFixed(1) }}</span>
          <span v-if="movie.playable" class="play-badge"><el-icon><VideoPlay /></el-icon></span>
        </div>
        <div class="poster-meta">
          <strong>{{ movie.title || movie.file_name }}</strong>
          <span class="pos-hint">{{ movie.year || '—' }} · {{ formatDuration(movie.runtime_seconds) }}</span>
        </div>
      </button>
    </div>

    <div v-if="movies.length" class="load-more">
      <el-button v-if="movies.length < total" :loading="loading" @click="loadMore">{{ t('photos.loadingMore') }}</el-button>
    </div>

    <!-- detail -->
    <el-drawer v-model="detailOpen" size="560px" :with-header="false">
      <div v-if="detail" class="movie-detail">
        <div class="detail-hero">
          <img v-if="detail.poster_url" :src="posterURL(detail, 500)" class="detail-poster" :alt="detail.title" />
          <div v-else class="detail-poster poster-fallback"><span>{{ (detail.title || '?').slice(0, 1) }}</span></div>
          <div class="detail-info">
            <div class="row">
              <h2>{{ detail.title }}</h2>
              <el-tag size="small" effect="plain">{{ kindLabel(detail.kind) }}</el-tag>
            </div>
            <p class="original">{{ detail.original_title }}</p>
            <div class="meta-line">
              <span v-if="detail.year">{{ detail.year }}</span>
              <span v-if="detail.rating"><el-icon><StarFilled /></el-icon> {{ detail.rating.toFixed(1) }}</span>
              <span>{{ formatDuration(detail.runtime_seconds) }}</span>
              <span v-if="detail.resolution">{{ detail.resolution }}</span>
            </div>
            <div class="genres">
              <el-tag v-for="g in genreList(detail.genres)" :key="g" size="small" effect="plain" type="info">{{ g }}</el-tag>
            </div>
            <div class="detail-buttons">
              <el-button type="primary" :icon="VideoPlay" :disabled="!detail.playable" @click="play(detail)">
                {{ t('movies.play') }}
              </el-button>
              <el-button :icon="Edit" @click="openEdit(detail)">{{ t('movies.edit') }}</el-button>
              <el-button :icon="MagicStick" @click="scrapeOne(detail)">{{ t('movies.scrape') }}</el-button>
              <el-button type="danger" plain :icon="Delete" @click="openDelete(detail)">{{ t('common.delete') }}</el-button>
            </div>
            <div v-if="!detail.playable" class="pos-hint">{{ t('movies.noFile') }}</div>
          </div>
        </div>

        <section>
          <h4>{{ t('movies.overview') }}</h4>
          <p class="overview">{{ detail.overview || '—' }}</p>
        </section>

        <section>
          <h4>{{ t('movies.cast') }}</h4>
          <div class="cast">
            <el-tag v-for="name in castList(detail.cast)" :key="name" size="small" effect="plain">{{ name }}</el-tag>
            <span v-if="!detail.cast" class="pos-hint">—</span>
          </div>
        </section>

        <section>
          <h4>{{ t('movies.subtitles') }}</h4>
          <div v-if="subtitles.length" class="subs">
            <el-tag v-for="sub in subtitles" :key="sub.name" size="small">{{ sub.name }}</el-tag>
          </div>
          <span v-else class="pos-hint">{{ t('movies.noSubtitles') }}</span>
        </section>

        <section class="tech">
          <h4>{{ t('movies.details') }}</h4>
          <el-descriptions :column="2" size="small" border>
            <el-descriptions-item :label="t('movies.resolution')">{{ detail.resolution || '—' }}</el-descriptions-item>
            <el-descriptions-item :label="t('movies.runtime')">{{ formatDuration(detail.runtime_seconds) }}</el-descriptions-item>
            <el-descriptions-item :label="t('movies.codecs')">{{ detail.video_codec || '—' }} / {{ detail.audio_codec || '—' }}</el-descriptions-item>
            <el-descriptions-item :label="t('movies.rating')">{{ detail.rating || 0 }}</el-descriptions-item>
          </el-descriptions>
        </section>
      </div>
    </el-drawer>

    <!-- player -->
    <el-dialog v-model="playerOpen" :title="playing?.title || t('movies.play')" width="min(1080px, 94vw)" destroy-on-close>
      <div v-if="playing" class="player-wrap">
        <video ref="videoEl" class="player" controls autoplay playsinline :src="streamURL(playing)">
          <track
            v-for="(sub, idx) in subtitles"
            :key="sub.name"
            :src="subURL(playing, sub.name)"
            :label="sub.name"
            :default="idx === 0"
            kind="subtitles"
            srclang="zh"
          />
        </video>
        <div class="player-controls">
          <span class="pos-hint">{{ t('movies.playHint') }}</span>
          <div class="pos-spacer" />
          <span class="pos-hint">{{ t('common.more') }}</span>
          <el-select v-model="speed" size="small" style="width: 96px" @change="setSpeed">
            <el-option :label="'0.5×'" :value="0.5" />
            <el-option :label="'0.75×'" :value="0.75" />
            <el-option :label="'1.0×'" :value="1" />
            <el-option :label="'1.25×'" :value="1.25" />
            <el-option :label="'1.5×'" :value="1.5" />
            <el-option :label="'2.0×'" :value="2" />
          </el-select>
        </div>
      </div>
    </el-dialog>

    <!-- TMDB settings -->
    <el-dialog v-model="tmdbOpen" :title="t('movies.tmdbSettings')" width="460px">
      <el-alert :title="t('movies.tmdbHint')" type="info" :closable="false" show-icon style="margin-bottom: 14px" />
      <el-form label-position="top">
        <el-form-item :label="t('movies.apiKey')">
          <el-input v-model="tmdbForm.api_key" show-password placeholder="TMDB API Key" />
        </el-form-item>
        <el-form-item :label="t('movies.language')">
          <el-select v-model="tmdbForm.language" style="width: 100%">
            <el-option label="简体中文" value="zh-CN" />
            <el-option label="English" value="en-US" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="tmdbOpen = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="busy" @click="saveTmdb">{{ t('movies.saveTmdb') }}</el-button>
      </template>
    </el-dialog>

    <!-- edit metadata -->
    <el-dialog v-model="editOpen" :title="t('movies.editTitle')" width="520px">
      <el-form label-position="top">
        <el-form-item :label="t('movies.titleField')">
          <el-input v-model="editForm.title" />
        </el-form-item>
        <div class="form-row">
          <el-form-item :label="t('movies.yearField')">
            <el-input-number v-model="editForm.year" :min="0" :max="2100" style="width: 100%" />
          </el-form-item>
          <el-form-item :label="t('movies.kindField')">
            <el-select v-model="editForm.kind" style="width: 100%">
              <el-option :label="t('movies.movie')" value="movie" />
              <el-option :label="t('movies.series')" value="series" />
              <el-option :label="t('movies.anime')" value="anime" />
              <el-option :label="t('movies.documentary')" value="documentary" />
            </el-select>
          </el-form-item>
        </div>
        <el-form-item :label="t('movies.overviewField')">
          <el-input v-model="editForm.overview" type="textarea" :rows="4" />
        </el-form-item>
        <el-form-item :label="t('movies.genresField')">
          <el-input v-model="editForm.genres" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editOpen = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="busy" @click="saveEdit">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- delete -->
    <el-dialog v-model="deleteOpen" :title="t('common.delete')" width="430px">
      <p>{{ t('movies.deleteConfirm') }}</p>
      <el-checkbox v-if="auth.isAdmin" v-model="deleteFile">{{ t('movies.deleteFile') }}</el-checkbox>
      <template #footer>
        <el-button @click="deleteOpen = false">{{ t('common.cancel') }}</el-button>
        <el-button type="danger" :loading="busy" @click="confirmDelete">{{ t('common.delete') }}</el-button>
      </template>
    </el-dialog>

    <!-- job progress -->
    <el-dialog v-model="jobOpen" :title="jobTitle" width="420px" :show-close="false" :close-on-click-modal="false">
      <el-progress :percentage="jobProgress" :status="jobStatus === 'failed' ? 'exception' : (jobStatus === 'done' ? 'success' : undefined)" />
      <div class="pos-hint" style="margin-top: 10px">{{ jobMessage }}</div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import {
  Refresh, RefreshRight, MagicStick, Setting, Search, VideoPlay, StarFilled, Edit, Delete
} from '@element-plus/icons-vue'
import PageHeader from '../components/PageHeader.vue'
import EmptyState from '../components/EmptyState.vue'
import { api, apiURL, errorMessage } from '../api'
import { useAuthStore } from '../stores/auth'
import { formatDuration } from '../utils/format'

const { t } = useI18n()
const auth = useAuthStore()

const movies = ref([])
const total = ref(0)
const loading = ref(false)
const scanning = ref(false)
const scraping = ref(false)
const page = ref(1)
const pageSize = 40
const meta = ref({ years: [], kinds: {} })
const filters = reactive({ q: '', kind: '', year: '', sort: 'title' })

const detailOpen = ref(false)
const detail = ref(null)
const subtitles = ref([])
const playerOpen = ref(false)
const playing = ref(null)
const videoEl = ref(null)
const speed = ref(1)

const tmdbOpen = ref(false)
const tmdbForm = reactive({ api_key: '', language: 'zh-CN' })

const editOpen = ref(false)
const editForm = reactive({ title: '', year: 0, kind: 'movie', overview: '', genres: '' })
const editing = ref(null)

const deleteOpen = ref(false)
const deleteFile = ref(false)
const deleting = ref(null)

const jobOpen = ref(false)
const jobTitle = ref('')
const jobProgress = ref(0)
const jobMessage = ref('')
const jobStatus = ref('running')
const busy = ref(false)

function posterURL(movie, size = 400) {
  const raw = movie.poster_url || `/movies/${movie.id}/poster`
  const rel = raw.replace(/^\/api\/v1/, '').split('?')[0]
  return apiURL(`${rel}?size=${size}`)
}

function streamURL(movie) {
  return apiURL(`/movies/${movie.id}/stream`)
}

function subURL(movie, name) {
  return apiURL(`/movies/${movie.id}/subtitles/${encodeURIComponent(name)}`)
}

function kindLabel(kind) {
  const map = { movie: 'movies.movie', series: 'movies.series', anime: 'movies.anime', documentary: 'movies.documentary' }
  return map[kind] ? t(map[kind]) : kind
}

function genreList(genres) {
  if (!genres) return []
  return genres.split(',').map((g) => g.trim()).filter(Boolean)
}

function castList(cast) {
  if (!cast) return []
  return cast.split(',').map((c) => c.trim()).filter(Boolean)
}

async function load(append = false) {
  loading.value = true
  try {
    const params = { page: page.value, page_size: pageSize, sort: filters.sort || 'title' }
    if (filters.q) params.q = filters.q
    if (filters.kind) params.kind = filters.kind
    if (filters.year) params.year = filters.year
    const payload = await api.movies.list(params)
    movies.value = append ? [...movies.value, ...payload.data] : payload.data
    total.value = payload.pagination.total
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function loadMeta() {
  try {
    meta.value = await api.movies.meta()
  } catch (error) {
    meta.value = { years: [], kinds: {} }
  }
}

function reload() {
  page.value = 1
  load(false)
  loadMeta()
}

function applyFilters() {
  page.value = 1
  load(false)
}

function loadMore() {
  page.value += 1
  load(true)
}

async function openDetail(movie) {
  try {
    const payload = await api.movies.get(movie.id)
    detail.value = payload.movie
    subtitles.value = payload.subtitles || []
    detailOpen.value = true
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

function play(movie) {
  playing.value = movie
  speed.value = 1
  playerOpen.value = true
}

function setSpeed(value) {
  if (videoEl.value) videoEl.value.playbackRate = value
}

async function pollJob(job, title) {
  if (!job?.id) return
  jobOpen.value = true
  jobTitle.value = title
  jobProgress.value = 0
  jobMessage.value = t('common.loading')
  jobStatus.value = 'running'
  for (let i = 0; i < 1800; i += 1) {
    await new Promise((resolve) => setTimeout(resolve, 1000))
    try {
      const jobs = await api.jobs.list()
      const current = jobs.find((j) => j.id === job.id)
      if (current) {
        jobProgress.value = current.progress || 0
        jobMessage.value = current.message || ''
        jobStatus.value = current.status
        if (current.status !== 'running') {
          if (current.status === 'failed') ElMessage.error(current.error || t('common.operationFailed'))
          else ElMessage.success(current.message || t('common.operationSuccess'))
          setTimeout(() => { jobOpen.value = false }, 900)
          return
        }
      }
    } catch (error) {
      jobOpen.value = false
      return
    }
  }
  jobOpen.value = false
}

async function scan() {
  scanning.value = true
  try {
    const job = await api.movies.scan()
    ElMessage.success(t('movies.scanStarted'))
    await pollJob(job, t('movies.scan'))
    await reload()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    scanning.value = false
  }
}

async function scrapeAll() {
  if (!meta.value.tmdb?.configured) {
    ElMessage.warning(t('movies.tmdbMissing'))
    if (auth.isAdmin) openTmdb()
    return
  }
  scraping.value = true
  try {
    const job = await api.movies.scrape([])
    ElMessage.success(t('movies.scrapeStarted'))
    await pollJob(job, t('movies.scrape'))
    await reload()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    scraping.value = false
  }
}

async function scrapeOne(movie) {
  if (!meta.value.tmdb?.configured) {
    ElMessage.warning(t('movies.tmdbMissing'))
    return
  }
  try {
    const job = await api.movies.scrape([movie.id])
    await pollJob(job, t('movies.scrape'))
    const payload = await api.movies.get(movie.id)
    detail.value = payload.movie
    subtitles.value = payload.subtitles || []
    load(false)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

function openTmdb() {
  tmdbForm.api_key = ''
  tmdbForm.language = meta.value.tmdb?.language || 'zh-CN'
  tmdbOpen.value = true
}

async function saveTmdb() {
  busy.value = true
  try {
    await api.movies.setTmdb({ api_key: tmdbForm.api_key || undefined, language: tmdbForm.language })
    ElMessage.success(t('movies.settingsSaved'))
    tmdbOpen.value = false
    await loadMeta()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

function openEdit(movie) {
  editing.value = movie
  editForm.title = movie.title || ''
  editForm.year = movie.year || 0
  editForm.kind = movie.kind || 'movie'
  editForm.overview = movie.overview || ''
  editForm.genres = movie.genres || ''
  editOpen.value = true
}

async function saveEdit() {
  busy.value = true
  try {
    await api.movies.update(editing.value.id, {
      title: editForm.title,
      year: editForm.year,
      kind: editForm.kind,
      overview: editForm.overview,
      genres: editForm.genres
    })
    ElMessage.success(t('movies.updateSuccess'))
    editOpen.value = false
    await reload()
    if (detail.value?.id === editing.value.id) {
      const payload = await api.movies.get(editing.value.id)
      detail.value = payload.movie
    }
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

function openDelete(movie) {
  deleting.value = movie
  deleteFile.value = false
  deleteOpen.value = true
}

async function confirmDelete() {
  busy.value = true
  try {
    await api.movies.remove(deleting.value.id, deleteFile.value)
    ElMessage.success(t('movies.removeSuccess'))
    deleteOpen.value = false
    detailOpen.value = false
    await reload()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

onMounted(() => {
  reload()
})
onBeforeUnmount(() => {
  if (videoEl.value) videoEl.value.pause()
})
</script>

<style scoped>
.filters {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  flex-wrap: wrap;
}
.search { width: 220px; flex: 0 0 220px; }
.poster-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(168px, 1fr));
  gap: 16px;
  min-height: 200px;
}
.poster-card {
  background: transparent;
  border: none;
  padding: 0;
  cursor: pointer;
  text-align: left;
  color: var(--pos-text);
}
.poster {
  position: relative;
  aspect-ratio: 2 / 3;
  border-radius: 13px;
  overflow: hidden;
  background: var(--pos-panel-2);
  border: 1px solid var(--pos-border);
  transition: transform 0.18s ease, border-color 0.18s ease, box-shadow 0.18s ease;
}
.poster-card:hover .poster {
  transform: translateY(-4px);
  border-color: rgba(45, 212, 191, 0.45);
  box-shadow: 0 16px 34px rgba(0, 0, 0, 0.35);
}
.poster img { width: 100%; height: 100%; object-fit: cover; display: block; }
.poster-fallback {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background:
    radial-gradient(300px 200px at 20% 10%, rgba(45, 212, 191, 0.24), transparent 70%),
    linear-gradient(150deg, #152532, #0d1720);
  color: rgba(230, 237, 243, 0.5);
  font-size: 46px;
  font-weight: 700;
}
.kind-tag {
  position: absolute;
  top: 8px;
  left: 8px;
  font-size: 10px;
  padding: 2px 7px;
  border-radius: 999px;
  background: rgba(8, 15, 20, 0.7);
  border: 1px solid rgba(139, 161, 180, 0.25);
  color: #d7e2ec;
  backdrop-filter: blur(4px);
}
.rating {
  position: absolute;
  top: 8px;
  right: 8px;
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-size: 11px;
  color: #fbbf24;
  background: rgba(8, 15, 20, 0.72);
  padding: 2px 7px;
  border-radius: 999px;
}
.play-badge {
  position: absolute;
  bottom: 8px;
  right: 8px;
  width: 34px;
  height: 34px;
  border-radius: 50%;
  background: rgba(45, 212, 191, 0.9);
  color: #06202a;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transform: scale(0.9);
  transition: all 0.18s ease;
}
.poster-card:hover .play-badge { opacity: 1; transform: scale(1); }
.poster-meta {
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 8px 3px 0;
}
.poster-meta strong {
  font-size: 12.5px;
  font-weight: 550;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.load-more { display: flex; justify-content: center; padding: 8px 0 24px; }
.movie-detail { display: flex; flex-direction: column; gap: 22px; padding: 6px 4px; }
.detail-hero { display: flex; gap: 18px; }
.detail-poster {
  width: 170px;
  aspect-ratio: 2 / 3;
  border-radius: 14px;
  object-fit: cover;
  border: 1px solid var(--pos-border);
  flex-shrink: 0;
}
.detail-info { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 8px; }
.detail-info .row { display: flex; align-items: flex-start; gap: 10px; }
.detail-info h2 { margin: 0; font-size: 22px; line-height: 1.3; }
.original { margin: 0; color: var(--pos-muted); font-size: 12px; }
.meta-line { display: flex; gap: 12px; flex-wrap: wrap; font-size: 12px; color: var(--pos-muted); }
.meta-line span { display: inline-flex; align-items: center; gap: 4px; }
.genres { display: flex; gap: 6px; flex-wrap: wrap; }
.detail-buttons { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 4px; }
h4 { margin: 0 0 8px; font-size: 13px; color: var(--pos-muted); letter-spacing: 0.4px; }
.overview { margin: 0; font-size: 13px; line-height: 1.8; color: var(--pos-text); opacity: 0.9; }
.cast { display: flex; gap: 6px; flex-wrap: wrap; }
.subs { display: flex; gap: 6px; flex-wrap: wrap; }
.player { width: 100%; max-height: 70vh; background: #000; border-radius: 10px; }
.player-controls { display: flex; align-items: center; gap: 10px; padding-top: 10px; }
.form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
</style>
