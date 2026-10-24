<template>
  <div class="pos-page files-page">
    <PageHeader :title="t('files.title')" :subtitle="t('files.subtitle')">
      <el-button :icon="CopyDocument" @click="openNewWindow">{{ t('desktop.newWindow') }}</el-button>
      <el-button :icon="Operation" @click="layout.toggle('files')">{{ t('files.sidebar') }}</el-button>
      <el-radio-group v-model="viewMode" size="small">
        <el-radio-button value="list"><el-icon><List /></el-icon></el-radio-button>
        <el-radio-button value="grid"><el-icon><Grid /></el-icon></el-radio-button>
        <el-radio-button value="columns"><el-icon><Operation /></el-icon></el-radio-button>
      </el-radio-group>
      <el-button :icon="View" :type="previewVisible ? 'primary' : 'default'" @click="togglePreview">{{ t('files.previewSidebar') }}</el-button>
      <el-button :icon="Refresh" :loading="loading" @click="reload">{{ t('common.refresh') }}</el-button>
    </PageHeader>

    <div class="files-layout" :style="{ '--files-sidebar-width': layout.sidebars.files.visible ? `${layout.sidebars.files.width}px` : '0px', '--files-preview-width': previewVisible ? `${previewWidth}px` : '0px' }">
      <aside v-show="layout.sidebars.files.visible" class="pos-panel tree-panel">
        <SidebarResizer
          v-if="layout.sidebars.files.visible"
          side="left"
          :width="layout.sidebars.files.width"
          @resize="(w) => layout.setWidth('files', w)"
        />
        <div class="tree-head">
          <span class="pos-card-title">{{ t('files.myFiles') }}</span>
          <el-tag v-if="canSeeAll" size="small" effect="plain" type="info">{{ t('users.admin') }}</el-tag>
        </div>
        <el-tree
          ref="treeRef"
          :key="treeKey"
          :props="{ label: 'name', isLeaf: 'leaf' }"
          lazy
          :load="loadNode"
          node-key="path"
          highlight-current
          :expand-on-click-node="false"
          @node-click="onTreeNodeClick"
        >
          <template #default="{ data }">
            <span class="tree-node">
              <el-icon :size="15"><Folder v-if="data.isDir" /><Document v-else /></el-icon>
              <span>{{ data.name }}</span>
            </span>
          </template>
        </el-tree>

        <div class="tree-extra">
          <button class="tree-action" :class="{ active: trashMode }" type="button" @click="openTrash">
            <el-icon><Delete /></el-icon><span>{{ t('files.trash') }}</span>
            <el-tag v-if="trashCount" size="small" type="info">{{ trashCount }}</el-tag>
          </button>
          <el-switch
            v-if="canSeeAll"
            v-model="seeAll"
            :active-text="t('files.allUsers')"
            size="small"
            @change="switchScope"
          />
        </div>
      </aside>

      <section class="pos-panel content-panel" @dragover.prevent="dragging = true" @dragleave.prevent="dragging = false" @drop.prevent="onDrop">
        <div class="content-head">
          <div class="breadcrumbs">
            <button type="button" class="crumb" @click="goTo('')">
              <el-icon><HomeFilled /></el-icon>
              {{ seeAll ? t('files.allUsers') : t('files.myFiles') }}
            </button>
            <template v-for="(part, idx) in pathParts" :key="idx">
              <span class="sep">/</span>
              <button type="button" class="crumb" @click="goTo(part.path)">{{ part.name }}</button>
            </template>
          </div>
          <div class="pos-spacer" />
          <el-input
            v-model="searchQuery"
            class="search"
            :placeholder="t('files.searchPlaceholder')"
            clearable
            :prefix-icon="Search"
            @input="onSearchInput"
            @clear="clearSearch"
          />
        </div>

        <div v-if="!trashMode" class="toolbar">
          <el-button type="primary" :icon="Upload" @click="pickFiles">{{ t('files.uploadFile') }}</el-button>
          <el-button :icon="FolderAdd" @click="showNewFolder">{{ t('files.newFolder') }}</el-button>
          <el-button :icon="Download" :disabled="!selection.length" @click="downloadSelected">{{ t('common.download') }}</el-button>
          <el-button :icon="Share" :disabled="selection.length !== 1" @click="openShare">{{ t('files.share') }}</el-button>
          <el-button :icon="Rank" :disabled="!selection.length" @click="openMove('move')">{{ t('files.move') }}</el-button>
          <el-button :icon="CopyDocument" :disabled="!selection.length" @click="openMove('copy')">{{ t('files.copy') }}</el-button>
          <el-button :icon="Box" :disabled="!archivesSelected" @click="extractSelected">{{ t('files.extract') }}</el-button>
          <el-button :icon="Rank" :disabled="!selection.length" @click="openBatchRename">{{ t('files.batchRename') }}</el-button>
          <el-button type="danger" plain :icon="Delete" :disabled="!selection.length" @click="removeSelected">
            {{ t('common.delete') }}
          </el-button>
          <div class="pos-spacer" />
          <span v-if="selection.length" class="pos-hint">{{ t('files.selected', { n: selection.length }) }}</span>
        </div>

        <div v-else class="toolbar">
          <el-button :icon="ArrowLeft" @click="trashMode = false">{{ t('common.back') }}</el-button>
          <el-button :disabled="!selection.length" @click="restoreSelected">{{ t('files.restore') }}</el-button>
          <el-button type="danger" plain @click="emptyTrash">{{ t('files.emptyTrash') }}</el-button>
          <div class="pos-spacer" />
          <span class="pos-hint">{{ t('files.trashHint') }}</span>
        </div>

        <div v-if="dragging" class="drop-overlay">
          <el-icon :size="36"><UploadFilled /></el-icon>
          <strong>{{ t('files.dropHint') }}</strong>
        </div>

        <!-- Search results -->
        <template v-if="searchQuery && searched">
          <div class="result-head">{{ t('files.searchResults') }} · {{ searchResults.length }}</div>
          <div v-if="!searchResults.length" class="pos-hint" style="padding: 24px">{{ t('files.noSearchResults') }}</div>
          <div v-else class="grid-view">
            <div v-for="item in searchResults" :key="item.path" class="grid-card" @click="openEntry(item)">
              <FileIcon :type="item.type" :name="item.name" :is-dir="item.is_dir" :size="24" />
              <div class="grid-name">{{ item.name }}</div>
              <div class="grid-sub">{{ item.is_dir ? t('files.folder') : formatBytes(item.size) }}</div>
            </div>
          </div>
        </template>

        <!-- Trash -->
        <template v-else-if="trashMode">
          <div v-if="!trashItems.length" class="empty-wrap">
            <EmptyState :title="t('files.trashEmpty')" :description="t('files.trashHint')" icon="Delete" />
          </div>
          <div v-else class="table-area">
          <el-table
            :data="trashItems"
            @selection-change="onSelectionChange"
            row-key="id"
            height="100%"
          >
            <el-table-column type="selection" width="46" />
            <el-table-column :label="t('files.name')" min-width="240">
              <template #default="{ row }">
                <div class="name-cell">
                  <FileIcon :type="row.is_dir ? 'folder' : 'file'" :name="row.name" :is-dir="row.is_dir" />
                  <span>{{ row.name }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column :label="t('files.size')" width="120">
              <template #default="{ row }">{{ row.is_dir ? '—' : formatBytes(row.size_bytes) }}</template>
            </el-table-column>
            <el-table-column :label="t('files.modified')" width="180">
              <template #default="{ row }">{{ formatDate(row.deleted_at) }}</template>
            </el-table-column>
            <el-table-column :label="t('files.trashRetention')" width="140">
              <template #default="{ row }">{{ row.expires_at ? formatDate(row.expires_at, 'YYYY-MM-DD') : '—' }}</template>
            </el-table-column>
          </el-table>
          </div>
        </template>

        <!-- List view -->
        <template v-else-if="viewMode === 'list'">
          <div class="table-area">
          <el-table
            :data="sortedEntries"
            v-loading="loading"
            height="100%"
            @selection-change="onSelectionChange"
            @row-dblclick="openEntry"
            @row-click="onRowPreview"
            @row-contextmenu="(row, column, event) => openContextMenu(event, row)"
            row-key="path"
          >
            <el-table-column type="selection" width="46" />
            <el-table-column :label="t('files.name')" min-width="280" sortable prop="name">
              <template #default="{ row }">
                <div class="name-cell">
                  <FileIcon :type="row.type" :name="row.name" :is-dir="row.is_dir" />
                  <button type="button" class="name-link" @click.stop="openEntry(row)">{{ row.name }}</button>
                </div>
              </template>
            </el-table-column>
            <el-table-column :label="t('files.size')" width="120" sortable prop="size">
              <template #default="{ row }">{{ row.is_dir ? '—' : formatBytes(row.size) }}</template>
            </el-table-column>
            <el-table-column :label="t('files.modified')" width="170" sortable prop="modified">
              <template #default="{ row }">{{ formatDate(row.modified) }}</template>
            </el-table-column>
            <el-table-column :label="t('files.owner')" width="150" show-overflow-tooltip>
              <template #default="{ row }">{{ row.owner || '—' }}</template>
            </el-table-column>
            <el-table-column width="120" align="right">
              <template #default="{ row }">
                <el-dropdown trigger="click" @command="(cmd) => rowAction(cmd, row)">
                  <el-button link :icon="MoreFilled" />
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="open">{{ t('common.open') }}</el-dropdown-item>
                      <el-dropdown-item command="download" v-if="!row.is_dir">{{ t('common.download') }}</el-dropdown-item>
                      <el-dropdown-item command="rename">{{ t('files.rename') }}</el-dropdown-item>
                      <el-dropdown-item command="share">{{ t('files.share') }}</el-dropdown-item>
                      <el-dropdown-item command="details">{{ t('common.details') }}</el-dropdown-item>
                      <el-dropdown-item divided command="delete">{{ t('common.delete') }}</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </template>
            </el-table-column>
          </el-table>
          </div>
        </template>

        <!-- Column view -->
        <template v-else-if="viewMode === 'columns'">
          <div class="column-view">
            <div class="column-list">
              <button
                v-for="item in sortedEntries"
                :key="item.path"
                type="button"
                class="column-item"
                :class="{ active: columnEntry?.path === item.path }"
                @click="onColumnSelect(item)"
                @dblclick="openEntry(item)"
                @contextmenu.prevent="openContextMenu($event, item)"
              >
                <FileIcon :type="item.type" :name="item.name" :is-dir="item.is_dir" :size="19" />
                <span>{{ item.name }}</span>
              </button>
            </div>
            <div class="column-detail">
              <template v-if="columnEntry">
                <template v-if="columnEntry.is_dir">
                  <div class="column-head">{{ columnEntry.name }}</div>
                  <div v-if="columnLoading" class="pos-hint" style="padding: 16px">{{ t('common.loading') }}</div>
                  <button v-for="child in columnEntries" :key="child.path" type="button" class="column-item" @click="onColumnSelect(child)" @dblclick="openEntry(child)">
                    <FileIcon :type="child.type" :name="child.name" :is-dir="child.is_dir" :size="17" />
                    <span>{{ child.name }}</span>
                  </button>
                  <div v-if="!columnEntries.length && !columnLoading" class="pos-hint" style="padding: 16px">{{ t('files.emptyFolder') }}</div>
                </template>
                <FilePreview v-else :entry="columnEntry" />
              </template>
              <div v-else class="pos-hint" style="padding: 20px">{{ t('files.columnHint') }}</div>
            </div>
          </div>
        </template>

        <!-- Grid view -->
        <template v-else>
          <div ref="fileGrid" v-loading="loading" class="grid-view" @mousedown.self="startBand">
            <div
              v-for="item in sortedEntries"
              :key="item.path"
              class="grid-card"
              :data-path="item.path"
              :class="{ selected: selection.some((s) => s.path === item.path) }"
              @click="onGridClick(item, $event)"
              @dblclick="openEntry(item)"
              @contextmenu.prevent="openContextMenu($event, item)"
            >
              <div class="grid-thumb">
                <el-image
                  v-if="item.thumbnail && !item.is_dir"
                  :src="thumbURL(item)"
                  fit="cover"
                  lazy
                  :preview-src-list="[thumbURL(item, 800)]"
                  preview-teleported
                />
                <FileIcon v-else :type="item.type" :name="item.name" :is-dir="item.is_dir" :size="30" />
              </div>
              <div class="grid-name">{{ item.name }}</div>
              <div class="grid-sub">{{ item.is_dir ? t('files.folder') : formatBytes(item.size) }}</div>
            </div>
            <div v-if="band.active" class="rubber-band" :style="{ left: `${band.x}px`, top: `${band.y}px`, width: `${band.w}px`, height: `${band.h}px` }" />
            <div v-if="!sortedEntries.length && !loading" class="empty-wrap">
              <EmptyState :title="t('files.emptyFolder')" :description="t('files.emptyFolderHint')" icon="FolderOpened">
                <el-button type="primary" :icon="Upload" @click="pickFiles">{{ t('files.uploadFile') }}</el-button>
              </EmptyState>
            </div>
          </div>
        </template>

        <div class="content-foot">
          <span>{{ t('files.itemsCount', { n: trashMode ? trashItems.length : sortedEntries.length }) }}</span>
          <div class="pos-spacer" />
          <span class="pos-hint">{{ formatBytes(usage.used_bytes || 0) }} / {{ usage.quota_bytes ? formatBytes(usage.quota_bytes) : t('common.unlimited') }}</span>
        </div>
      </section>

      <aside v-if="previewVisible" class="preview-panel">
        <SidebarResizer side="right" :width="previewWidth" @resize="setPreviewWidth" />
        <div class="preview-panel-head">
          <strong>{{ t('files.preview') }}</strong>
          <button type="button" @click="previewVisible = false"><el-icon><Close /></el-icon></button>
        </div>
        <FilePreview v-if="previewEntry" :entry="previewEntry" />
        <div v-else class="pos-hint" style="padding: 16px">{{ t('files.previewHint') }}</div>
      </aside>
    </div>

    <input ref="fileInput" type="file" multiple hidden @change="onFilePicked" />

    <!-- context menu -->
    <Teleport to="body">
    <div v-if="ctx.visible" class="context-menu" :style="{ left: `${ctx.x}px`, top: `${ctx.y}px` }">
      <button type="button" @click="ctxAction('open')">{{ t('common.open') }}</button>
      <button v-if="ctx.entry && !ctx.entry.is_dir" type="button" @click="ctxAction('download')">{{ t('common.download') }}</button>
      <button type="button" @click="ctxAction('rename')">{{ t('files.rename') }}</button>
      <button type="button" @click="ctxAction('share')">{{ t('files.share') }}</button>
      <button type="button" @click="ctxAction('details')">{{ t('common.details') }}</button>
      <button type="button" class="danger" @click="ctxAction('delete')">{{ t('common.delete') }}</button>
    </div>
    </Teleport>

    <!-- details drawer -->
    <el-drawer v-model="detailsOpen" :title="t('files.detailsTitle')" size="380px">
      <div v-if="detailEntry" class="details">
        <div class="details-icon">
          <FileIcon :type="detailEntry.type" :name="detailEntry.name" :is-dir="detailEntry.is_dir" :size="38" />
        </div>
        <h3>{{ detailEntry.name }}</h3>
        <el-descriptions :column="1" border size="small">
          <el-descriptions-item :label="t('files.type')">{{ detailEntry.is_dir ? t('files.folder') : detailEntry.mime || detailEntry.type }}</el-descriptions-item>
          <el-descriptions-item :label="t('files.size')">{{ formatBytes(detailEntry.size) }}</el-descriptions-item>
          <el-descriptions-item :label="t('files.modified')">{{ formatDate(detailEntry.modified) }}</el-descriptions-item>
          <el-descriptions-item :label="t('files.owner')">{{ detailEntry.owner || '—' }}</el-descriptions-item>
          <el-descriptions-item :label="t('files.permissions')"><span class="pos-mono">{{ detailEntry.mode }}</span></el-descriptions-item>
          <el-descriptions-item :label="t('files.path')"><span class="pos-mono">{{ detailEntry.path }}</span></el-descriptions-item>
        </el-descriptions>
        <div class="details-preview">
          <el-image v-if="isImage(detailEntry)" :src="previewURL(detailEntry)" fit="contain" />
          <video v-else-if="isVideo(detailEntry)" :src="previewURL(detailEntry)" controls />
          <div v-else class="pos-hint">{{ t('files.noPreview') }}</div>
        </div>
        <div class="details-actions">
          <el-button :icon="Download" @click="downloadEntry(detailEntry)">{{ t('common.download') }}</el-button>
          <el-button :icon="Share" @click="shareEntry(detailEntry)">{{ t('files.share') }}</el-button>
        </div>
      </div>
    </el-drawer>

    <!-- new folder -->
    <el-dialog v-model="newFolderOpen" :title="t('files.newFolderTitle')" width="420px">
      <el-input v-model="newFolderName" :placeholder="t('files.folderName')" @keyup.enter="createFolder" />
      <template #footer>
        <el-button @click="newFolderOpen = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="busy" @click="createFolder">{{ t('common.create') }}</el-button>
      </template>
    </el-dialog>

    <!-- move / copy -->
    <el-dialog v-model="moveOpen" :title="moveMode === 'move' ? t('files.moveTitle') : t('files.copyTitle')" width="480px">
      <el-form-item :label="t('files.targetPath')">
        <el-input v-model="moveTarget" placeholder="Documents/2025" />
      </el-form-item>
      <el-tree-select
        v-model="moveTarget"
        :data="folderOptions"
        :props="{ label: 'name', value: 'path', children: 'children' }"
        check-strictly
        :render-after-expand="false"
        :placeholder="t('files.selectFolder')"
        style="width: 100%"
      />
      <template #footer>
        <el-button @click="moveOpen = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="busy" @click="submitMove">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- share dialog -->
    <el-dialog v-model="shareOpen" :title="t('files.shareTitle')" width="460px">
      <template v-if="!shareLink">
        <el-form label-position="top">
          <el-form-item :label="t('files.sharePassword')">
            <el-input v-model="shareForm.password" show-password />
          </el-form-item>
          <el-form-item :label="t('files.shareExpiry')">
            <el-input-number v-model="shareForm.hours" :min="0" :max="8760" style="width: 100%" />
          </el-form-item>
          <el-form-item :label="t('files.shareMaxDownloads')">
            <el-input-number v-model="shareForm.maxDownloads" :min="0" :max="100000" style="width: 100%" />
          </el-form-item>
        </el-form>
      </template>
      <div v-else class="share-result">
        <el-result icon="success" :title="t('files.shareCreated')">
          <template #extra>
            <div class="share-link">
              <el-input :model-value="shareLink" readonly />
              <el-button type="primary" @click="copyShareLink">{{ t('common.copy') }}</el-button>
            </div>
          </template>
        </el-result>
      </div>
      <template #footer>
        <el-button @click="shareOpen = false">{{ t('common.close') }}</el-button>
        <el-button v-if="!shareLink" type="primary" :loading="busy" @click="submitShare">{{ t('files.createShare') }}</el-button>
      </template>
    </el-dialog>

    <!-- upload progress -->

    <Teleport to="body">
      <div v-if="quicklookOpen && previewEntry" class="quicklook" @click.self="quicklookOpen = false">
        <div class="quicklook-card">
          <div class="quicklook-head">
            <strong>{{ previewEntry.name }}</strong>
            <button type="button" @click="quicklookOpen = false"><el-icon><Close /></el-icon></button>
          </div>
          <FilePreview :entry="previewEntry" />
        </div>
      </div>
    </Teleport>

    <el-dialog v-model="batchOpen" :title="t('files.batchRename')" width="560px">
      <el-form label-position="top">
        <el-form-item :label="t('files.renameMode')">
          <el-radio-group v-model="batchForm.mode">
            <el-radio-button value="prefix">{{ t('files.addPrefix') }}</el-radio-button>
            <el-radio-button value="suffix">{{ t('files.addSuffix') }}</el-radio-button>
            <el-radio-button value="insert">{{ t('files.insertAt') }}</el-radio-button>
            <el-radio-button value="replace">{{ t('files.replaceText') }}</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <div class="batch-grid">
          <el-form-item v-if="batchForm.mode === 'replace'" :label="t('files.findText')">
            <el-input v-model="batchForm.find" />
          </el-form-item>
          <el-form-item :label="batchForm.mode === 'replace' ? t('files.replaceWith') : t('files.text')">
            <el-input v-model="batchForm.text" />
          </el-form-item>
          <el-form-item v-if="batchForm.mode === 'insert'" :label="t('files.insertPosition')">
            <el-input-number v-model="batchForm.position" :min="1" :max="120" />
          </el-form-item>
        </div>
        <div class="batch-preview">
          <div v-for="row in batchPreviewNames" :key="row.from" class="batch-row"><span>{{ row.from }}</span><span>→</span><strong>{{ row.to }}</strong></div>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="batchOpen = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="busy" :disabled="!selection.length" @click="applyBatchRename">{{ t('files.applyRename') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="uploadOpen" :title="t('files.uploadFile')" width="420px" :close-on-click-modal="false" :show-close="false">
      <div class="upload-box">
        <el-progress :percentage="uploadPercent" :status="uploadPercent >= 100 ? 'success' : undefined" />
        <div class="pos-hint">{{ uploadLabel }}</div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Refresh, Upload, FolderAdd, Download, Delete, Share, Rank, CopyDocument, Box,
  Search, List, Grid, MoreFilled, HomeFilled, Folder, Document, UploadFilled, ArrowLeft, Close
} from '@element-plus/icons-vue'
import PageHeader from '../components/PageHeader.vue'
import FileIcon from '../components/FileIcon.vue'
import EmptyState from '../components/EmptyState.vue'
import SidebarResizer from '../components/SidebarResizer.vue'
import FilePreview from '../components/FilePreview.vue'
import { api, apiURL, errorMessage } from '../api'
import { useUiStore } from '../stores/ui'
import { useDesktopStore } from '../stores/desktop'
import { useLayoutStore } from '../stores/layout'
import { useTaskStore } from '../stores/tasks'
import { formatBytes, formatDate, fileExt } from '../utils/format'

const { t } = useI18n()
const ui = useUiStore()
const desktop = useDesktopStore()
const layout = useLayoutStore()
const tasks = useTaskStore()

const treeRef = ref(null)
const treeKey = ref(0)
const fileInput = ref(null)
const loading = ref(false)
const busy = ref(false)
const currentPath = ref('')
const entries = ref([])
const usage = ref({})
const canSeeAll = ref(false)
const seeAll = ref(false)
const viewMode = ref(localStorage.getItem('pos_files_view') || 'list')
const selection = ref([])
const searchQuery = ref('')
const searchResults = ref([])
const searched = ref(false)
const trashMode = ref(false)
const trashItems = ref([])
const trashCount = ref(0)
const dragging = ref(false)
const detailsOpen = ref(false)
const detailEntry = ref(null)
const newFolderOpen = ref(false)
const newFolderName = ref('')
const moveOpen = ref(false)
const moveMode = ref('move')
const moveTarget = ref('')
const folderOptions = ref([])
const shareOpen = ref(false)
const shareTarget = ref(null)
const shareForm = reactive({ password: '', hours: 0, maxDownloads: 0 })
const shareLink = ref('')
const uploadOpen = ref(false)
const uploadPercent = ref(0)
const previewVisible = ref(localStorage.getItem('pos_files_preview') === '1')
const previewWidth = ref(Number(localStorage.getItem('pos_files_preview_width') || 300))
const previewEntry = ref(null)
const quicklookOpen = ref(false)
const columnEntry = ref(null)
const columnEntries = ref([])
const columnLoading = ref(false)
const batchOpen = ref(false)
const batchForm = reactive({ mode: 'prefix', text: '', position: 1, find: '', replace: '' })
const fileGrid = ref(null)
const band = reactive({ active: false, x: 0, y: 0, w: 0, h: 0, startX: 0, startY: 0, moved: false })
const uploadLabel = ref('')
const ctx = reactive({ visible: false, x: 0, y: 0, entry: null })
let searchTimer = null

const pathParts = computed(() => {
  if (!currentPath.value) return []
  const parts = currentPath.value.split('/').filter(Boolean)
  return parts.map((name, idx) => ({ name, path: parts.slice(0, idx + 1).join('/') }))
})

const sortedEntries = computed(() => [...entries.value])

const archivesSelected = computed(() =>
  selection.value.length > 0 && selection.value.every((e) => !e.is_dir && ['zip', 'tar', 'gz', 'tgz', '7z', 'rar', 'xz', 'bz2'].includes(fileExt(e.name)))
)

function scopeParams() {
  return seeAll.value ? { scope: 'all' } : {}
}

function thumbURL(item, size = 240) {
  return apiURL(`/files/thumb?${new URLSearchParams({ path: item.path, size: String(size), ...scopeParams() })}`)
}

function previewURL(item) {
  if (isImage(item)) return apiURL(`/files/download?${new URLSearchParams({ path: item.path, ...scopeParams() })}`)
  return apiURL(`/files/download?${new URLSearchParams({ path: item.path, ...scopeParams() })}`)
}

function isImage(item) {
  return ['image'].includes(item.type) || ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp'].includes(fileExt(item.name))
}
function isVideo(item) {
  return ['video'].includes(item.type) || ['mp4', 'webm', 'mkv', 'mov'].includes(fileExt(item.name))
}

async function reload() {
  if (trashMode.value) {
    await loadTrash()
    return
  }
  await load(currentPath.value)
}

async function load(path) {
  loading.value = true
  try {
    const payload = await api.files.list({ path, ...scopeParams() })
    const listing = payload.listing
    entries.value = listing.entries || []
    usage.value = listing.usage || {}
    canSeeAll.value = payload.can_see_all
    currentPath.value = listing.path || ''
    selection.value = []
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function loadTrash() {
  loading.value = true
  try {
    trashItems.value = await api.files.trash()
    trashCount.value = trashItems.value.length
    selection.value = []
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    loading.value = false
  }
}

function goTo(path) {
  searchQuery.value = ''
  searched.value = false
  load(path)
}

function onTreeNodeClick(data) {
  if (data.isDir) goTo(data.path)
}

async function loadNode(node, resolve) {
  if (node.level === 0) {
    try {
      const payload = await api.files.list({ path: '', ...scopeParams() })
      canSeeAll.value = payload.can_see_all
      resolve((payload.listing.entries || []).filter((e) => e.is_dir).map((e) => ({ name: e.name, path: e.path, isDir: true })))
    } catch (error) {
      resolve([])
    }
    return
  }
  try {
    const payload = await api.files.list({ path: node.data.path, ...scopeParams() })
    resolve((payload.listing.entries || []).filter((e) => e.is_dir).map((e) => ({
      name: e.name, path: e.path, isDir: true, leaf: false
    })))
  } catch (error) {
    resolve([])
  }
}

function switchScope() {
  currentPath.value = ''
  searchQuery.value = ''
  searched.value = false
  treeKey.value += 1
  load('')
}

function openTrash() {
  trashMode.value = true
  loadTrash()
}

function onSelectionChange(rows) {
  selection.value = rows
  if (rows.length === 1) previewEntry.value = rows[0]
  else if (!rows.length) previewEntry.value = null
}

function togglePreview() {
  previewVisible.value = !previewVisible.value
  localStorage.setItem('pos_files_preview', previewVisible.value ? '1' : '0')
}
function setPreviewWidth(w) {
  previewWidth.value = Math.max(220, Math.min(520, Math.round(w)))
  localStorage.setItem('pos_files_preview_width', String(previewWidth.value))
}
function onRowPreview(row) {
  previewEntry.value = row
}
function onGridClick(item, event) {
  toggleRow(item, event)
  previewEntry.value = item
}
async function onColumnSelect(item) {
  columnEntry.value = item
  previewEntry.value = item
  columnEntries.value = []
  if (!item.is_dir) return
  columnLoading.value = true
  try {
    const payload = await api.files.list({ path: item.path, ...scopeParams() })
    columnEntries.value = payload.listing?.entries || []
  } catch (error) {
    columnEntries.value = []
  } finally {
    columnLoading.value = false
  }
}
function openBatchRename() {
  if (!selection.value.length) return
  batchForm.mode = 'prefix'
  batchForm.text = ''
  batchForm.position = 1
  batchForm.find = ''
  batchOpen.value = true
}
function buildBatchName(name) {
  const dot = name.lastIndexOf('.')
  const ext = dot > 0 ? name.slice(dot) : ''
  const base = dot > 0 ? name.slice(0, dot) : name
  if (batchForm.mode === 'prefix') return batchForm.text + name
  if (batchForm.mode === 'suffix') return base + batchForm.text + ext
  if (batchForm.mode === 'insert') {
    const pos = Math.max(1, Math.min(name.length + 1, Number(batchForm.position) || 1)) - 1
    return name.slice(0, pos) + batchForm.text + name.slice(pos)
  }
  if (batchForm.mode === 'replace') {
    if (!batchForm.find) return name
    return name.split(batchForm.find).join(batchForm.replace)
  }
  return name
}
const batchPreviewNames = computed(() => selection.value.map((item) => ({ from: item.name, to: buildBatchName(item.name) })))
async function applyBatchRename() {
  if (!selection.value.length) return
  if (batchForm.mode === 'replace' && !batchForm.find) {
    ElMessage.warning(t('files.findText'))
    return
  }
  busy.value = true
  let done = 0
  const task = tasks.add(t('files.batchRename'), t('files.selected', { n: selection.value.length }))
  try {
    for (const item of selection.value) {
      const name = buildBatchName(item.name)
      if (name && name !== item.name) {
        await api.files.rename({ path: item.path, name })
        done++
      }
    }
    tasks.finish(task)
    ElMessage.success(t('files.batchRenameDone', { n: done }))
    batchOpen.value = false
    await load(currentPath.value)
  } catch (error) {
    tasks.finish(task, 'failed', errorMessage(error))
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}
function startBand(event) {
  if (event.button !== 0) return
  band.active = true
  band.moved = false
  band.startX = event.clientX
  band.startY = event.clientY
  band.x = event.clientX
  band.y = event.clientY
  band.w = 0
  band.h = 0
  const onMove = (e) => {
    const x = Math.min(band.startX, e.clientX)
    const y = Math.min(band.startY, e.clientY)
    band.x = x
    band.y = y
    band.w = Math.abs(e.clientX - band.startX)
    band.h = Math.abs(e.clientY - band.startY)
    if (band.w > 4 || band.h > 4) band.moved = true
  }
  const onUp = () => {
    window.removeEventListener('pointermove', onMove)
    window.removeEventListener('pointerup', onUp)
    if (band.moved && fileGrid.value) {
      const rect = { left: band.x, top: band.y, right: band.x + band.w, bottom: band.y + band.h }
      const picked = []
      fileGrid.value.querySelectorAll('.grid-card').forEach((el) => {
        const r = el.getBoundingClientRect()
        if (r.left < rect.right && r.right > rect.left && r.top < rect.bottom && r.bottom > rect.top) {
          const item = sortedEntries.value.find((x) => x.path === el.dataset.path)
          if (item) picked.push(item)
        }
      })
      selection.value = picked
      if (picked.length === 1) previewEntry.value = picked[0]
    }
    band.active = false
  }
  window.addEventListener('pointermove', onMove)
  window.addEventListener('pointerup', onUp)
}
function openQuicklook() {
  if (selection.value.length === 1) previewEntry.value = selection.value[0]
  if (previewEntry.value) quicklookOpen.value = true
}

function toggleRow(row, event) {
  if (event?.ctrlKey || event?.metaKey) {
    const idx = selection.value.findIndex((s) => s.path === row.path)
    if (idx >= 0) selection.value.splice(idx, 1)
    else selection.value.push(row)
  } else {
    selection.value = [row]
  }
}

function openEntry(item) {
  if (item.is_dir) {
    goTo(item.path)
  } else {
    detailEntry.value = item
    detailsOpen.value = true
  }
}

function rowAction(cmd, row) {
  if (cmd === 'open') openEntry(row)
  else if (cmd === 'download') downloadEntry(row)
  else if (cmd === 'rename') renameEntry(row)
  else if (cmd === 'share') shareEntry(row)
  else if (cmd === 'details') { detailEntry.value = row; detailsOpen.value = true }
  else if (cmd === 'delete') deleteEntries([row])
}

function pickFiles() {
  fileInput.value?.click()
}

async function onFilePicked(event) {
  const files = Array.from(event.target.files || [])
  event.target.value = ''
  await uploadFiles(files)
}

async function onDrop(event) {
  dragging.value = false
  const files = Array.from(event.dataTransfer?.files || [])
  await uploadFiles(files)
}

async function uploadFiles(files) {
  if (!files.length) return
  uploadOpen.value = true
  uploadPercent.value = 0
  uploadLabel.value = files[0].name
  const form = new FormData()
  form.append('path', currentPath.value)
  files.forEach((file) => form.append('files', file, file.name))
  try {
    await api.files.upload(form, (evt) => {
      if (evt.total) uploadPercent.value = Math.round((evt.loaded / evt.total) * 100)
    })
    uploadPercent.value = 100
    ElMessage.success(t('files.uploadSuccess'))
    setTimeout(() => { uploadOpen.value = false }, 500)
    await load(currentPath.value)
    treeKey.value += 1
    window.dispatchEvent(new CustomEvent('pos:refresh-notifications'))
  } catch (error) {
    uploadOpen.value = false
    ElMessage.error(errorMessage(error))
  }
}

function showNewFolder() {
  newFolderName.value = ''
  newFolderOpen.value = true
}

async function createFolder() {
  const name = newFolderName.value.trim()
  if (!name) return
  busy.value = true
  try {
    await api.files.mkdir({ path: currentPath.value, name })
    ElMessage.success(t('files.createFolderSuccess'))
    newFolderOpen.value = false
    await load(currentPath.value)
    treeKey.value += 1
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

function downloadEntry(item) {
  if (item.is_dir) {
    const params = new URLSearchParams({ path: item.path, ...scopeParams() })
    window.open(apiURL(`/files/archive?${params}`), '_blank')
  } else {
    const params = new URLSearchParams({ path: item.path, ...scopeParams() })
    window.open(apiURL(`/files/download?${params}`), '_blank')
  }
  ElMessage.success(t('files.downloadStarted'))
}

function downloadSelected() {
  const params = new URLSearchParams(scopeParams())
  selection.value.forEach((item) => params.append('path', item.path))
  window.open(apiURL(`/files/archive?${params}`), '_blank')
}

async function renameEntry(item) {
  try {
    const { value } = await ElMessageBox.prompt(t('files.renameTitle'), item.name, {
      inputValue: item.name,
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel')
    })
    if (!value || value === item.name) return
    await api.files.rename({ path: item.path, name: value })
    ElMessage.success(t('common.operationSuccess'))
    await load(currentPath.value)
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(errorMessage(error))
  }
}

function openMove(mode) {
  moveMode.value = mode
  moveTarget.value = currentPath.value
  const children = []
  const walk = (items, base) => {
    items.filter((e) => e.is_dir).forEach((e) => {
      const node = { name: e.name, path: e.path, children: [] }
      children.push(node)
    })
  }
  walk(entries.value)
  folderOptions.value = [{ name: seeAll.value ? t('files.allUsers') : t('files.myFiles'), path: '', children }]
  moveOpen.value = true
}

async function submitMove() {
  if (!selection.value.length) return
  busy.value = true
  try {
    const payload = { paths: selection.value.map((e) => e.path), destination: moveTarget.value }
    const task = tasks.add(moveMode.value === 'move' ? t('files.move') : t('files.copy'), `${selection.value.length} → ${moveTarget.value}`)
    try {
      if (moveMode.value === 'move') await api.files.move(payload)
      else await api.files.copy(payload)
      tasks.finish(task)
    } catch (taskError) {
      tasks.finish(task, 'failed', errorMessage(taskError))
      throw taskError
    }
    ElMessage.success(t('common.operationSuccess'))
    moveOpen.value = false
    await load(currentPath.value)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

async function deleteEntries(items) {
  try {
    await ElMessageBox.confirm(
      t('files.deleteConfirm', { n: items.length }),
      t('common.delete'),
      {
        type: 'warning',
        description: t('files.deleteHint'),
        confirmButtonText: t('common.delete'),
        cancelButtonText: t('common.cancel')
      }
    )
  } catch (error) {
    return
  }
  try {
    const task = tasks.add(t('common.delete'), `${items.length} ${t('files.selected', { n: items.length })}`)
    try {
      await api.files.remove({ paths: items.map((e) => e.path) })
      tasks.finish(task)
    } catch (taskError) {
      tasks.finish(task, 'failed', errorMessage(taskError))
      throw taskError
    }
    ElMessage.success(t('files.deleteSuccess') || t('common.deleteSuccess'))
    selection.value = []
    await load(currentPath.value)
    await loadTrash()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

function removeSelected() {
  deleteEntries(selection.value)
}

async function restoreSelected() {
  try {
    await api.files.restore(selection.value.map((e) => e.id))
    ElMessage.success(t('common.operationSuccess'))
    await loadTrash()
    await load(currentPath.value)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

async function emptyTrash() {
  try {
    await ElMessageBox.confirm(t('files.emptyTrashConfirm'), t('files.emptyTrash'), {
      type: 'warning',
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel')
    })
  } catch (error) {
    return
  }
  try {
    await api.files.emptyTrash([])
    ElMessage.success(t('files.trashCleared'))
    await loadTrash()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

async function extractSelected() {
  const target = selection.value[0]
  if (!target) return
  busy.value = true
  try {
    const result = await api.files.extract({ path: target.path, destination: currentPath.value })
    ElMessage.success(t('files.extractSuccess'))
    await load(result.path || currentPath.value)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

function shareEntry(item) {
  shareTarget.value = item
  shareForm.password = ''
  shareForm.hours = 0
  shareForm.maxDownloads = 0
  shareLink.value = ''
  shareOpen.value = true
}

function openShare() {
  if (selection.value.length === 1) shareEntry(selection.value[0])
}

async function submitShare() {
  busy.value = true
  try {
    const payload = await api.files.share({
      path: shareTarget.value.path,
      name: shareTarget.value.name,
      password: shareForm.password || undefined,
      expires_hours: shareForm.hours,
      max_downloads: shareForm.maxDownloads
    })
    shareLink.value = `${window.location.origin}/s/${payload.share.token}`
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

async function copyShareLink() {
  try {
    await navigator.clipboard.writeText(shareLink.value)
    ElMessage.success(t('common.copied'))
  } catch (error) {
    const el = document.createElement('textarea')
    el.value = shareLink.value
    document.body.appendChild(el)
    el.select()
    document.execCommand('copy')
    document.body.removeChild(el)
    ElMessage.success(t('common.copied'))
  }
}

function onSearchInput() {
  if (searchTimer) clearTimeout(searchTimer)
  if (!searchQuery.value.trim()) {
    searched.value = false
    searchResults.value = []
    return
  }
  searchTimer = setTimeout(async () => {
    try {
      searchResults.value = await api.files.search({ q: searchQuery.value, ...scopeParams() })
      searched.value = true
    } catch (error) {
      ElMessage.error(errorMessage(error))
    }
  }, 350)
}

function clearSearch() {
  searched.value = false
  searchResults.value = []
}

function openContextMenu(event, row) {
  const entry = row || selection.value[0]
  if (!entry) return
  ctx.entry = entry
  ctx.x = Math.min(event.clientX, window.innerWidth - 210)
  ctx.y = Math.min(event.clientY, window.innerHeight - 240)
  ctx.visible = true
}

function ctxAction(cmd) {
  ctx.visible = false
  const entry = ctx.entry
  if (!entry) return
  if (cmd === 'open') openEntry(entry)
  else if (cmd === 'download') downloadEntry(entry)
  else if (cmd === 'rename') renameEntry(entry)
  else if (cmd === 'share') shareEntry(entry)
  else if (cmd === 'details') { detailEntry.value = entry; detailsOpen.value = true }
  else if (cmd === 'delete') deleteEntries([entry])
}

function openNewWindow() {
  desktop.open('files', { forceNew: true })
}

function closeContext() {
  ctx.visible = false
}

function onKeydown(event) {
  if (event.key === 'Escape') {
    closeContext()
    quicklookOpen.value = false
    return
  }
  if (event.code === 'Space') {
    const tag = (event.target.tagName || '').toLowerCase()
    const type = (event.target.type || '').toLowerCase()
    if ((tag === 'input' || tag === 'textarea') && type !== 'checkbox' && type !== 'radio') return
    if (event.target.isContentEditable) return
    if (selection.value.length === 1 || previewEntry.value) {
      event.preventDefault()
      openQuicklook()
    }
  }
}
onMounted(() => {
  window.addEventListener('click', closeContext)
  window.addEventListener('keydown', onKeydown)
  load('')
  loadTrash().catch(() => {})
})
onBeforeUnmount(() => {
  window.removeEventListener('click', closeContext)
  window.removeEventListener('keydown', onKeydown)
})
</script>

<style scoped>
.files-page { height: calc(100dvh - 106px); min-height: 520px; }
.files-layout {
  flex: 1;
  display: grid;
  grid-template-columns: var(--files-sidebar-width, 248px) minmax(0, 1fr) var(--files-preview-width, 0px);
  gap: 0;
  min-height: 0;
  border: 1px solid var(--pos-border);
  background: var(--pos-panel);
}
.tree-panel {
  position: relative;
  padding: 16px 12px;
  display: flex;
  flex-direction: column;
  min-height: 0;
  background: var(--pos-nav);
  border: none;
  border-right: 1px solid var(--pos-border);
  border-radius: 0;
  box-shadow: none;
}
.tree-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 6px 10px;
}
.tree-panel :deep(.el-tree) {
  --el-tree-node-hover-bg-color: rgba(139, 161, 180, 0.1);
  background: transparent;
  color: var(--pos-text);
  font-size: 13px;
  flex: 1;
  overflow: auto;
}
.tree-panel :deep(.el-tree-node.is-current > .el-tree-node__content) {
  background: var(--pos-accent-soft);
  color: var(--pos-accent);
  border-radius: 8px;
}
.tree-node { display: inline-flex; align-items: center; gap: 7px; }
.tree-extra {
  border-top: 1px solid var(--pos-border);
  padding-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.tree-action {
  display: flex;
  align-items: center;
  gap: 8px;
  background: transparent;
  border: none;
  color: var(--pos-muted);
  padding: 8px 10px;
  border-radius: 9px;
  cursor: pointer;
  font-size: 12.5px;
}
.tree-action:hover, .tree-action.active { background: var(--pos-accent-soft); color: var(--pos-accent); }
.tree-action .el-tag { margin-left: auto; }
.table-area {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  overflow: hidden;
}
.table-area :deep(.el-table) {
  flex: 1;
  width: 100%;
}
.content-panel {
  padding: 14px 16px;
  min-width: 0;
  background: var(--pos-panel);
  border: none;
  border-radius: 0;
  box-shadow: none;
  display: flex;
  flex-direction: column;
  min-height: 0;
  position: relative;
}
.content-head {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 10px;
}
.breadcrumbs {
  display: flex;
  align-items: center;
  gap: 3px;
  flex-wrap: wrap;
  min-width: 0;
}
.crumb {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  background: transparent;
  border: none;
  color: var(--pos-muted);
  font-size: 12.5px;
  cursor: pointer;
  padding: 3px 6px;
  border-radius: 7px;
}
.crumb:hover { color: var(--pos-accent); background: var(--pos-accent-soft); }
.sep { color: var(--pos-border); }
.search { width: 240px; }
.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--pos-border);
  margin-bottom: 8px;
}
.toolbar :deep(.el-button) { margin-left: 0; }
.name-cell { display: flex; align-items: center; gap: 10px; min-width: 0; }
.name-cell span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.name-link {
  background: transparent;
  border: none;
  color: var(--pos-text);
  cursor: pointer;
  font-size: 13px;
  padding: 0;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.name-link:hover { color: var(--pos-accent); }
.grid-view {
  position: relative;
  flex: 1;
  overflow: auto;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 12px;
  padding: 10px 2px;
  align-content: start;
  min-height: 0;
}
.rubber-band { position: fixed; z-index: 8800; border: 1px solid var(--pos-accent); background: color-mix(in srgb, var(--pos-accent) 12%, transparent); pointer-events: none; }
.grid-card {
  border: 1px solid var(--pos-border);
  border-radius: 12px;
  padding: 12px;
  background: var(--pos-panel-2);
  display: flex;
  flex-direction: column;
  gap: 7px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.grid-card:hover { border-color: rgba(45, 212, 191, 0.4); transform: translateY(-2px); }
.grid-card.selected { border-color: var(--pos-accent); background: var(--pos-accent-soft); }
.grid-thumb {
  height: 92px;
  border-radius: 9px;
  background: rgba(139, 161, 180, 0.08);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}
.grid-thumb .el-image { width: 100%; height: 100%; }
.grid-name { font-size: 12.5px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.grid-sub { font-size: 10.5px; color: var(--pos-muted); }
.result-head { font-size: 12px; color: var(--pos-muted); padding: 6px 2px; }
.empty-wrap { grid-column: 1 / -1; }
.content-foot {
  display: flex;
  align-items: center;
  padding-top: 8px;
  border-top: 1px solid var(--pos-border);
  font-size: 11.5px;
  color: var(--pos-muted);
  margin-top: auto;
}
.drop-overlay {
  position: absolute;
  inset: 12px;
  border: 2px dashed var(--pos-accent);
  border-radius: 14px;
  background: rgba(11, 15, 20, 0.86);
  display: flex;
  flex-direction: column;
  gap: 12px;
  align-items: center;
  justify-content: center;
  color: var(--pos-accent);
  z-index: 10;
}
.context-menu {
  position: fixed;
  z-index: 3000;
  min-width: 190px;
  background: var(--pos-panel);
  border: 1px solid var(--pos-border);
  border-radius: 0;
  box-shadow: var(--pos-shadow);
  padding: 6px;
  display: flex;
  flex-direction: column;
}
.context-menu button {
  border: none;
  background: transparent;
  text-align: left;
  padding: 8px 10px;
  border-radius: 7px;
  color: var(--pos-text);
  font-size: 12.5px;
  cursor: pointer;
}
.context-menu button:hover { background: var(--pos-accent-soft); color: var(--pos-accent); }
.context-menu button.danger:hover { background: rgba(248, 113, 113, 0.12); color: var(--pos-danger); }
.details { display: flex; flex-direction: column; gap: 12px; }
.details-icon {
  width: 68px;
  height: 68px;
  border-radius: 18px;
  background: var(--pos-panel-2);
  display: flex;
  align-items: center;
  justify-content: center;
}
.details h3 { margin: 0; font-size: 16px; word-break: break-all; }
.details-preview {
  border-radius: 12px;
  overflow: hidden;
  background: var(--pos-panel-2);
  min-height: 120px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 10px;
}
.details-preview img, .details-preview video { max-width: 100%; max-height: 300px; border-radius: 8px; }
.details-actions { display: flex; gap: 10px; }
.share-result { padding: 4px 0; }
.share-link { display: flex; gap: 8px; }
.upload-box { display: flex; flex-direction: column; gap: 12px; padding: 10px 0; }

@media (max-width: 960px) {
  .files-page { height: auto; }
  .files-layout { grid-template-columns: minmax(0, 1fr); }
  .tree-panel { display: none; }
  .content-panel { min-height: 70vh; min-width: 0; overflow: hidden; }
  .search { width: 150px; }
}

.preview-panel {
  position: relative;
  min-width: 0;
  border-left: 1px solid var(--pos-border);
  background: var(--pos-panel);
  padding: 10px 12px;
  overflow: auto;
}
.preview-panel-head { display: flex; align-items: center; justify-content: space-between; padding-bottom: 7px; margin-bottom: 8px; border-bottom: 1px solid var(--pos-border); }
.preview-panel-head strong { font-size: 12px; }
.preview-panel-head button { border: none; background: transparent; color: var(--pos-muted); cursor: pointer; }
.column-view { flex: 1; min-height: 0; display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.1fr); border-top: 1px solid var(--pos-border); }
.column-list { min-width: 0; overflow: auto; border-right: 1px solid var(--pos-border); padding: 6px; }
.column-detail { min-width: 0; overflow: auto; padding: 10px; }
.column-head { font-size: 12px; font-weight: 600; padding: 4px 6px 8px; border-bottom: 1px solid var(--pos-border); margin-bottom: 4px; }
.column-item {
  display: flex;
  align-items: center;
  gap: 7px;
  width: 100%;
  padding: 7px 8px;
  border: none;
  border-radius: 0;
  background: transparent;
  color: var(--pos-text);
  font-size: 11.5px;
  cursor: pointer;
  text-align: left;
  min-width: 0;
}
.column-item span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.column-item:hover { background: var(--pos-panel-2); }
.column-item.active { background: var(--pos-accent-soft); color: var(--pos-accent); }
.quicklook { position: fixed; inset: 0; z-index: 8500; background: rgba(10, 16, 22, 0.55); display: flex; align-items: center; justify-content: center; }
.quicklook-card { width: 720px; max-width: calc(100vw - 40px); max-height: calc(100vh - 60px); overflow: auto; background: var(--pos-panel); border: 1px solid var(--pos-border-strong); padding: 14px; box-shadow: 0 24px 70px rgba(0,0,0,.28); }
.quicklook-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; }
.quicklook-head button { border: none; background: transparent; color: var(--pos-muted); cursor: pointer; }
.batch-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 0 12px; }
.batch-preview { max-height: 180px; overflow: auto; border: 1px solid var(--pos-border); margin-top: 4px; }
.batch-row { display: grid; grid-template-columns: minmax(0,1fr) 20px minmax(0,1fr); gap: 6px; padding: 5px 8px; border-bottom: 1px solid var(--pos-border); font-size: 11px; }
.batch-row span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.batch-row strong { font-weight: 550; color: var(--pos-accent); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
@media (max-width: 960px) {
  .preview-panel { display: none; }
}
</style>
