<template>
  <div class="pos-page">
    <PageHeader :title="t('settings.title')" :subtitle="t('settings.subtitle')">
      <el-tag effect="plain" :type="auth.pro ? 'warning' : 'info'">{{ auth.editionName }}</el-tag>
      <el-tag effect="plain">v{{ auth.version }}</el-tag>
    </PageHeader>

    <div class="settings-layout">
      <aside class="pos-panel side-nav">
        <button
          v-for="item in tabs"
          :key="item.name"
          type="button"
          class="side-item"
          :class="{ active: tab === item.name }"
          @click="tab = item.name"
        >
          <el-icon :size="16"><component :is="item.icon" /></el-icon>
          <span>{{ t(item.label) }}</span>
        </button>
      </aside>

      <section class="pos-panel main-panel" v-loading="loading">
        <!-- general -->
        <div v-if="tab === 'general'" class="section">
          <h3>{{ t('settings.tabs.general') }}</h3>
          <div class="grid-2">
            <div class="sub-card">
              <h4>{{ t('settings.general.systemName') }}</h4>
              <el-form label-position="top">
                <el-form-item :label="t('settings.general.systemName')">
                  <el-input v-model="general.name" />
                </el-form-item>
                <el-form-item :label="t('settings.general.website')">
                  <el-input v-model="general.website" />
                </el-form-item>
                <el-form-item :label="t('settings.general.language')">
                  <el-radio-group :model-value="ui.lang" @update:model-value="ui.setLang">
                    <el-radio-button value="zh-CN">简体中文</el-radio-button>
                    <el-radio-button value="en-US">English</el-radio-button>
                  </el-radio-group>
                </el-form-item>
                <el-form-item :label="t('settings.general.theme')">
                  <el-radio-group :model-value="ui.theme" @update:model-value="ui.setTheme">
                    <el-radio-button value="dark">{{ t('settings.general.dark') }}</el-radio-button>
                    <el-radio-button value="light">{{ t('settings.general.light') }}</el-radio-button>
                  </el-radio-group>
                </el-form-item>
                <el-button type="primary" :loading="busy" @click="saveGeneral">{{ t('common.save') }}</el-button>
              </el-form>
            </div>

            <div class="sub-card">
              <h4>{{ t('settings.general.systemInfo') }}</h4>
              <el-descriptions :column="1" size="small" border class="info-desc">
                <el-descriptions-item :label="t('settings.general.os')">{{ info.os || '—' }}</el-descriptions-item>
                <el-descriptions-item :label="t('settings.general.kernel')">{{ info.kernel || '—' }}</el-descriptions-item>
                <el-descriptions-item :label="t('settings.general.hostname')">{{ info.hostname || '—' }}</el-descriptions-item>
                <el-descriptions-item :label="t('settings.general.cpu')">{{ info.cpu_model || '—' }}</el-descriptions-item>
                <el-descriptions-item :label="t('settings.general.memory')">{{ formatBytes(info.mem_total || 0) }}</el-descriptions-item>
                <el-descriptions-item :label="t('settings.general.version')">{{ info.version || '—' }}</el-descriptions-item>
                <el-descriptions-item :label="t('settings.general.edition')">{{ info.edition_name || auth.editionName }}</el-descriptions-item>
                <el-descriptions-item :label="t('settings.general.uptime')">{{ formatUptime(info.uptime || 0) }}</el-descriptions-item>
                <el-descriptions-item :label="t('settings.general.dataDir')">
                  <span class="pos-mono">{{ paths.data }}</span>
                </el-descriptions-item>
              </el-descriptions>
              <div class="caps">
                <el-tooltip
                  v-for="(enabled, name) in info.capabilities || {}"
                  :key="name"
                  :content="enabled ? t('common.enabled') : t('common.disabled')"
                  placement="top"
                >
                  <el-tag :type="enabled ? 'success' : 'info'" size="small" effect="plain" class="cap-tag">{{ name }}</el-tag>
                </el-tooltip>
              </div>
            </div>
          </div>

          <div class="sub-card">
            <div class="sub-head">
              <h4>{{ t('settings.general.updates') }}</h4>
              <div class="row">
                <el-button size="small" :icon="Refresh" :loading="checkingUpdates" @click="checkUpdates">
                  {{ t('settings.general.checkUpdates') }}
                </el-button>
                <el-button
                  size="small"
                  type="primary"
                  :disabled="!updates.length"
                  :loading="applyingUpdates"
                  @click="applyUpdates"
                >
                  {{ t('settings.general.applyUpdates') }}
                </el-button>
              </div>
            </div>
            <el-alert
              v-if="!updates.length && updatesChecked"
              :title="t('settings.general.upToDate')"
              type="success"
              :closable="false"
              show-icon
            />
            <div v-else-if="updates.length" class="update-box">
              <span>{{ t('settings.general.updateAvailable', { n: updates.length }) }}</span>
              <el-table :data="updates" size="small" height="220">
                <el-table-column prop="name" :label="t('settings.general.packageName')" min-width="160" />
                <el-table-column prop="current" :label="t('settings.general.currentVersion')" width="180" />
                <el-table-column prop="new" :label="t('settings.general.newVersion')" width="180" />
              </el-table>
              <div class="pos-hint">{{ t('settings.general.rebootHint') }}</div>
            </div>
            <div v-else class="pos-hint">{{ t('common.loading') }}</div>
          </div>
        </div>

        <!-- account -->
        <div v-else-if="tab === 'account'" class="section">
          <h3>{{ t('settings.tabs.account') }}</h3>
          <div class="grid-2">
            <div class="sub-card">
              <h4>{{ t('settings.account.changePassword') }}</h4>
              <el-form label-position="top">
                <el-form-item :label="t('settings.account.oldPassword')">
                  <el-input v-model="passwordForm.old" type="password" show-password />
                </el-form-item>
                <el-form-item :label="t('settings.account.newPassword')">
                  <el-input v-model="passwordForm.next" type="password" show-password />
                </el-form-item>
                <el-form-item :label="t('settings.account.confirmPassword')">
                  <el-input v-model="passwordForm.confirm" type="password" show-password />
                </el-form-item>
                <el-button type="primary" :loading="busy" @click="changePassword">{{ t('common.save') }}</el-button>
              </el-form>
            </div>

            <div class="sub-card">
              <div class="sub-head">
                <h4>{{ t('settings.account.twoFactor') }}</h4>
                <el-tag :type="auth.user?.totp_enabled ? 'success' : 'info'" effect="plain">
                  {{ auth.user?.totp_enabled ? t('common.enabled') : t('common.disabled') }}
                </el-tag>
              </div>
              <template v-if="!auth.user?.totp_enabled">
                <p class="pos-hint">{{ t('settings.account.twoFactorOff') }}</p>
                <el-button v-if="!totpSetupData" type="primary" :icon="Key" :loading="busy" @click="setupTotp">
                  {{ t('settings.account.enable2fa') }}
                </el-button>
                <div v-else class="totp-setup">
                  <img v-if="qrCode" :src="qrCode" alt="TOTP QR" class="qr" />
                  <div class="totp-info">
                    <div class="pos-hint">{{ t('settings.account.scanQr') }}</div>
                    <div class="secret pos-mono">{{ totpSetupData.secret }}</div>
                    <el-input v-model="totpCode" maxlength="6" :placeholder="t('settings.account.verifyCode')" style="margin-top: 8px" />
                    <el-button type="primary" :loading="busy" style="margin-top: 8px" @click="enableTotp">
                      {{ t('settings.account.enable2fa') }}
                    </el-button>
                  </div>
                </div>
              </template>
              <template v-else>
                <p class="pos-hint">{{ t('settings.account.twoFactorOn') }}</p>
                <el-form label-position="top">
                  <el-form-item :label="t('settings.account.password')">
                    <el-input v-model="disableForm.password" type="password" show-password />
                  </el-form-item>
                  <el-form-item :label="t('settings.account.code')">
                    <el-input v-model="disableForm.code" maxlength="6" />
                  </el-form-item>
                  <el-button type="danger" plain :loading="busy" @click="disableTotp">
                    {{ t('settings.account.disable2fa') }}
                  </el-button>
                </el-form>
              </template>
            </div>
          </div>

          <div class="sub-card">
            <div class="sub-head">
              <h4>{{ t('settings.account.sessions') }}</h4>
              <el-button size="small" :icon="Refresh" @click="loadSessions">{{ t('common.refresh') }}</el-button>
            </div>
            <el-table :data="sessions" size="small">
              <el-table-column prop="ip" :label="t('logs.ip')" width="140" />
              <el-table-column prop="user_agent" :label="t('common.details')" min-width="220" show-overflow-tooltip />
              <el-table-column :label="t('settings.account.currentSession')" width="180">
                <template #default="{ row }">
                  <span class="pos-hint">{{ formatDate(row.last_seen) }}</span>
                </template>
              </el-table-column>
              <el-table-column width="100" align="right">
                <template #default="{ row }">
                  <el-button link type="danger" size="small" @click="revokeSession(row)">
                    {{ t('settings.account.revoke') }}
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </div>

        <!-- security -->
        <div v-else-if="tab === 'security'" class="section">
          <h3>{{ t('settings.tabs.security') }}</h3>
          <div class="sub-card">
            <h4>{{ t('settings.security.lockoutTitle') }}</h4>
            <div class="form-grid">
              <el-form-item :label="t('settings.security.lockoutEnabled')">
                <el-switch v-model="security.lockout_enabled" />
              </el-form-item>
              <el-form-item :label="t('settings.security.maxAttempts')">
                <el-input-number v-model="security.max_attempts" :min="3" :max="50" style="width: 100%" />
              </el-form-item>
              <el-form-item :label="t('settings.security.window')">
                <el-input-number v-model="security.window_seconds" :min="60" :max="86400" :step="60" style="width: 100%" />
              </el-form-item>
              <el-form-item :label="t('settings.security.lockSeconds')">
                <el-input-number v-model="security.lock_seconds" :min="60" :max="86400" :step="60" style="width: 100%" />
              </el-form-item>
              <el-form-item :label="t('settings.security.delayEnabled')">
                <el-switch v-model="security.delay_enabled" />
              </el-form-item>
              <el-form-item :label="t('settings.security.delayFrom')">
                <el-input-number v-model="security.delay_from" :min="1" :max="50" style="width: 100%" />
              </el-form-item>
              <el-form-item :label="t('settings.security.baseDelay')">
                <el-input-number v-model="security.base_delay_seconds" :min="1" :max="60" style="width: 100%" />
              </el-form-item>
              <el-form-item :label="t('settings.security.maxDelay')">
                <el-input-number v-model="security.max_delay_seconds" :min="1" :max="3600" style="width: 100%" />
              </el-form-item>
            </div>
            <el-button type="primary" :loading="busy" @click="saveSecurity">{{ t('settings.security.saveSecurity') }}</el-button>
          </div>

          <div class="sub-card">
            <div class="sub-head">
              <h4>{{ t('settings.security.httpsTitle') }}</h4>
              <el-tag :type="ssl.serving_tls ? 'success' : 'info'" effect="plain">
                {{ ssl.serving_tls ? t('common.enabled') : t('common.disabled') }}
              </el-tag>
            </div>
            <p class="pos-hint">{{ t('settings.security.httpsHint') }}</p>
            <div class="row">
              <el-input v-model="certDomain" :placeholder="t('settings.security.certDomain')" style="max-width: 260px" />
              <el-button :icon="Lock" :loading="busy" @click="generateCert">{{ t('settings.security.generateCert') }}</el-button>
            </div>
            <div v-if="ssl.cert_exists" class="pos-mono cert-path">{{ ssl.cert_path }}</div>
          </div>

          <div class="sub-card">
            <div class="sub-head">
              <h4>{{ t('settings.security.firewallTitle') }}</h4>
              <el-tag :type="firewall.active ? 'success' : 'info'" effect="plain">{{ firewall.active ? t('common.enabled') : t('common.disabled') }}</el-tag>
            </div>
            <p class="pos-hint">{{ t('settings.security.firewallHint') }}</p>
            <el-alert :title="t('settings.security.sshWarning')" type="warning" :closable="false" show-icon style="margin-bottom: 10px" />
            <el-input v-model="firewallRules" type="textarea" :rows="12" class="mono-input" />
            <div class="row" style="margin-top: 10px">
              <el-button type="primary" :loading="busy" @click="applyFirewall">{{ t('settings.security.applyFirewall') }}</el-button>
              <el-button v-if="firewallPending" type="success" @click="confirmFirewall">
                {{ t('settings.security.confirmKeep') }}
              </el-button>
            </div>
            <el-alert
              v-if="firewallPending"
              :title="t('settings.security.firewallConfirmHint')"
              type="warning"
              :closable="false"
              show-icon
              style="margin-top: 10px"
            />
          </div>
        </div>

        <!-- notifications -->
        <div v-else-if="tab === 'notifications'" class="section">
          <h3>{{ t('settings.notifications.title') }}</h3>
          <div class="sub-card">
            <div class="sub-head">
              <h4>{{ t('settings.notifications.smtp') }}</h4>
              <el-switch v-model="notifications.email_enabled" />
            </div>
            <div class="form-grid">
              <el-form-item :label="t('settings.notifications.host')">
                <el-input v-model="notifications.smtp_host" />
              </el-form-item>
              <el-form-item :label="t('settings.notifications.port')">
                <el-input-number v-model="notifications.smtp_port" :min="1" :max="65535" style="width: 100%" />
              </el-form-item>
              <el-form-item :label="t('settings.notifications.username')">
                <el-input v-model="notifications.smtp_user" />
              </el-form-item>
              <el-form-item :label="t('settings.notifications.password')">
                <el-input v-model="notifications.smtp_password" show-password />
              </el-form-item>
              <el-form-item :label="t('settings.notifications.from')">
                <el-input v-model="notifications.smtp_from" />
              </el-form-item>
              <el-form-item :label="t('settings.notifications.to')">
                <el-input v-model="notifications.smtp_to" />
              </el-form-item>
              <el-form-item :label="t('settings.notifications.tls')">
                <el-switch v-model="notifications.smtp_tls" />
              </el-form-item>
            </div>
          </div>

          <div class="sub-card">
            <div class="sub-head">
              <h4>{{ t('settings.notifications.telegram') }}</h4>
              <el-switch v-model="notifications.telegram_enabled" />
            </div>
            <div class="form-grid">
              <el-form-item :label="t('settings.notifications.telegramToken')">
                <el-input v-model="notifications.telegram_token" show-password />
              </el-form-item>
              <el-form-item :label="t('settings.notifications.chatId')">
                <el-input v-model="notifications.telegram_chat_id" />
              </el-form-item>
            </div>
          </div>

          <div class="sub-card">
            <div class="sub-head">
              <h4>{{ t('settings.notifications.bark') }}</h4>
              <el-switch v-model="notifications.bark_enabled" />
            </div>
            <el-form-item :label="t('settings.notifications.barkUrl')">
              <el-input v-model="notifications.bark_url" />
            </el-form-item>
            <el-form-item :label="t('settings.notifications.serverKey')">
              <el-input v-model="notifications.serverchan_key" show-password />
            </el-form-item>
            <div class="pos-hint">{{ t('network.unsavedSecrets') }}</div>
          </div>

          <div class="row">
            <el-button type="primary" :loading="busy" @click="saveNotifications">{{ t('settings.notifications.saveNotifications') }}</el-button>
            <el-button :loading="busy" @click="testNotification">{{ t('settings.notifications.testSend') }}</el-button>
          </div>
          <div v-if="testResults.length" class="test-results">
            <div v-for="item in testResults" :key="item.channel" class="test-row">
              <el-tag :type="item.ok ? 'success' : 'danger'" size="small">{{ item.channel }}</el-tag>
              <span class="pos-hint">{{ item.ok ? t('common.operationSuccess') : item.error }}</span>
            </div>
          </div>
        </div>

        <!-- backup -->
        <div v-else-if="tab === 'backup'" class="section">
          <h3>{{ t('settings.backup.title') }}</h3>
          <p class="pos-hint">{{ t('settings.backup.hint') }}</p>
          <div class="grid-2">
            <div class="sub-card">
              <h4>{{ t('settings.backup.downloadBackup') }}</h4>
              <el-button type="primary" :icon="Download" @click="downloadBackup">{{ t('settings.backup.downloadBackup') }}</el-button>
            </div>
            <div class="sub-card">
              <h4>{{ t('settings.backup.restoreBackup') }}</h4>
              <p class="pos-hint">{{ t('settings.backup.restoreHint') }}</p>
              <input ref="restoreInput" type="file" accept=".gz,.tgz,application/gzip" hidden @change="onRestorePicked" />
              <el-button :icon="Upload" @click="restoreInput?.click()">{{ t('settings.backup.selectFile') }}</el-button>
            </div>
          </div>
          <div class="sub-card">
            <h4>{{ t('settings.backup.autoBackup') }}</h4>
            <div class="form-grid">
              <el-form-item :label="t('settings.backup.autoBackup')">
                <el-switch v-model="backup.auto" />
              </el-form-item>
              <el-form-item :label="t('settings.backup.keepCount')">
                <el-input-number v-model="backup.keep" :min="1" :max="100" style="width: 100%" />
              </el-form-item>
            </div>
            <el-button type="primary" :loading="busy" @click="saveBackup">{{ t('common.save') }}</el-button>
          </div>
        </div>

        <!-- destroy -->
        <div v-else-if="tab === 'destroy'" class="section">
          <template v-if="!auth.pro">
            <el-alert :title="t('settings.destroy.proOnly')" type="warning" :closable="false" show-icon />
          </template>
          <template v-else>
            <h3>{{ t('settings.destroy.title') }}</h3>
            <el-alert :title="t('settings.destroy.hint')" type="error" :closable="false" show-icon />
            <div class="sub-card">
              <div class="form-grid">
                <el-form-item :label="t('settings.destroy.enabled')">
                  <el-switch v-model="destroy.enabled" />
                </el-form-item>
                <el-form-item :label="t('settings.destroy.threshold')">
                  <el-input-number v-model="destroy.threshold" :min="3" :max="50" style="width: 100%" />
                </el-form-item>
                <el-form-item :label="t('settings.destroy.window')">
                  <el-input-number v-model="destroy.window_seconds" :min="60" :max="86400" :step="60" style="width: 100%" />
                </el-form-item>
                <el-form-item :label="t('settings.destroy.method')">
                  <el-select v-model="destroy.method" style="width: 100%">
                    <el-option :label="t('settings.destroy.methodKey')" value="key_destroy" />
                    <el-option :label="t('settings.destroy.methodOverwrite')" value="overwrite" />
                    <el-option :label="t('settings.destroy.methodDelete')" value="delete" />
                  </el-select>
                </el-form-item>
                <el-form-item :label="t('settings.destroy.fallback')">
                  <el-select v-model="destroy.fallback" style="width: 100%">
                    <el-option :label="t('settings.destroy.fallbackNone')" value="none" />
                    <el-option :label="t('settings.destroy.fallbackOverwrite')" value="overwrite" />
                    <el-option :label="t('settings.destroy.fallbackDelete')" value="delete" />
                  </el-select>
                </el-form-item>
                <el-form-item :label="t('settings.destroy.vaultDevice')">
                  <el-input v-model="destroy.vault_device" placeholder="/dev/mapper/privacyos-vault" />
                </el-form-item>
                <el-form-item :label="t('settings.destroy.silent')">
                  <el-switch v-model="destroy.silent" />
                </el-form-item>
                <el-form-item :label="t('settings.destroy.notify')">
                  <el-switch v-model="destroy.notify_admin" />
                </el-form-item>
              </div>
              <el-button type="primary" :loading="busy" @click="saveDestroyPolicy">{{ t('settings.destroy.savePolicy') }}</el-button>
            </div>

            <div class="sub-card">
              <div class="sub-head">
                <h4>{{ t('settings.destroy.vaults') }}</h4>
                <el-tag :type="vaultInfo.available ? 'success' : 'info'" effect="plain">{{ t('settings.destroy.vaultAvailable') }}</el-tag>
              </div>
              <el-table :data="vaultInfo.vaults || []" size="small">
                <el-table-column prop="username" :label="t('users.username')" min-width="140" />
                <el-table-column :label="t('common.status')" width="140">
                  <template #default="{ row }">
                    <el-tag v-if="row.exists" :type="row.unlocked ? 'success' : 'warning'" size="small">
                      {{ row.unlocked ? t('common.on') : t('common.off') }}
                    </el-tag>
                    <span v-else class="pos-hint">{{ t('settings.destroy.noVault') }}</span>
                  </template>
                </el-table-column>
                <el-table-column :label="t('storage.capacity')" width="130">
                  <template #default="{ row }">{{ row.size_bytes ? formatBytes(row.size_bytes) : '—' }}</template>
                </el-table-column>
                <el-table-column width="260" align="right">
                  <template #default="{ row }">
                    <el-button v-if="!row.exists" link type="primary" size="small" @click="openVaultCreate(row)">
                      {{ t('settings.destroy.createVault') }}
                    </el-button>
                    <template v-else>
                      <el-button v-if="!row.unlocked" link type="primary" size="small" @click="vaultAction('unlock', row)">
                        {{ t('settings.destroy.unlockVault') }}
                      </el-button>
                      <el-button v-else link size="small" @click="vaultAction('lock', row)">
                        {{ t('settings.destroy.lockVault') }}
                      </el-button>
                    </template>
                  </template>
                </el-table-column>
              </el-table>
            </div>

            <div class="sub-card danger-zone">
              <h4>{{ t('settings.destroy.manualDestroy') }}</h4>
              <p class="pos-hint">{{ t('settings.destroy.manualDestroyHint') }}</p>
              <div class="row">
                <el-input v-model="destroyUsername" :placeholder="t('users.username')" style="max-width: 260px" />
                <el-button type="danger" :loading="busy" @click="manualDestroy">{{ t('settings.destroy.manualDestroy') }}</el-button>
              </div>
            </div>
          </template>
        </div>

        <!-- about -->
        <div v-else class="section about">
          <img src="/logo.svg" alt="PrivacyOS" class="about-logo" />
          <h2>PrivacyOS</h2>
          <p class="pos-hint">{{ t('app.tagline') }}</p>
          <el-descriptions :column="2" border size="small" class="about-table">
            <el-descriptions-item :label="t('settings.about.version')">{{ auth.version }}</el-descriptions-item>
            <el-descriptions-item :label="t('settings.about.edition')">{{ auth.editionName }}</el-descriptions-item>
            <el-descriptions-item :label="t('settings.about.license')">MIT</el-descriptions-item>
            <el-descriptions-item :label="t('settings.about.builtOn')">Debian 13.7 Trixie</el-descriptions-item>
            <el-descriptions-item :label="t('settings.about.website')" :span="2">{{ settings.system?.website || 'privacyos.local' }}</el-descriptions-item>
          </el-descriptions>
          <p class="slogan">{{ t('settings.about.slogan') }}</p>
        </div>
      </section>
    </div>

    <el-dialog v-model="vaultDialog" :title="t('settings.destroy.createVault')" width="400px">
      <el-form label-position="top">
        <el-form-item :label="t('settings.destroy.sizeGb')">
          <el-input-number v-model="vaultSize" :min="1" :max="65536" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="vaultDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="busy" @click="createVault">{{ t('common.create') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="applyOpen" :title="t('settings.general.applyUpdates')" width="420px" :show-close="false" :close-on-click-modal="false">
      <el-progress :percentage="applyProgress" :status="applyStatus === 'failed' ? 'exception' : undefined" />
      <div class="pos-hint" style="margin-top: 10px">{{ applyMessage }}</div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Setting, User, Lock, Bell, Download, Delete, Info, Refresh, Key, Upload
} from '@element-plus/icons-vue'
import QRCode from 'qrcode'
import PageHeader from '../components/PageHeader.vue'
import { api, apiURL, errorMessage } from '../api'
import { useAuthStore } from '../stores/auth'
import { useUiStore } from '../stores/ui'
import { formatBytes, formatDate, formatUptime } from '../utils/format'

const { t } = useI18n()
const auth = useAuthStore()
const ui = useUiStore()

const tabs = [
  { name: 'general', label: 'settings.tabs.general', icon: 'Setting' },
  { name: 'account', label: 'settings.tabs.account', icon: 'User' },
  { name: 'security', label: 'settings.tabs.security', icon: 'Lock' },
  { name: 'notifications', label: 'settings.tabs.notifications', icon: 'Bell' },
  { name: 'backup', label: 'settings.tabs.backup', icon: 'Download' },
  { name: 'destroy', label: 'settings.tabs.destroy', icon: 'Delete' },
  { name: 'about', label: 'settings.tabs.about', icon: 'Info' }
]

const tab = ref('general')
const loading = ref(false)
const busy = ref(false)
const settings = ref({})
const paths = ref({})
const info = ref({})
const updates = ref([])
const updatesChecked = ref(false)
const checkingUpdates = ref(false)
const sessions = ref([])

const general = reactive({ name: 'PrivacyOS', website: 'privacyos.local' })
const passwordForm = reactive({ old: '', next: '', confirm: '' })
const totpSetupData = ref(null)
const totpCode = ref('')
const qrCode = ref('')
const disableForm = reactive({ password: '', code: '' })
const security = reactive({})
const ssl = reactive({})
const certDomain = ref('')
const firewall = reactive({ active: false, status: '', default_ruleset: '' })
const firewallRules = ref('')
const firewallPending = ref(false)
const notifications = reactive({})
const testResults = ref([])
const backup = reactive({ auto: false, keep: 7, schedule: 'daily' })
const destroy = reactive({})
const vaultInfo = ref({ vaults: [], available: false })
const vaultDialog = ref(false)
const vaultSize = ref(50)
const vaultUser = ref('')
const destroyUsername = ref('')
const restoreInput = ref(null)
const applyingUpdates = ref(false)
const applyOpen = ref(false)
const applyProgress = ref(0)
const applyMessage = ref('')
const applyStatus = ref('running')

function fillForms() {
  const s = settings.value || {}
  general.name = s.system?.name || 'PrivacyOS'
  general.website = s.system?.website || 'privacyos.local'
  Object.assign(security, s.security || {})
  Object.assign(notifications, s.notifications || {})
  Object.assign(backup, s.backup || {})
  Object.assign(destroy, s.destroy || {})
  certDomain.value = s.https?.domain || info.value.hostname || ''
}

async function loadAll() {
  loading.value = true
  try {
    const payload = await api.settings.all()
    settings.value = payload.settings
    paths.value = payload.paths
    fillForms()
    const [sysInfo, sslInfo, fw] = await Promise.all([
      api.settings.info().catch(() => ({})),
      api.settings.ssl().catch(() => ({})),
      api.settings.firewall().catch(() => ({}))
    ])
    info.value = sysInfo.info || {}
    Object.assign(ssl, sslInfo)
    Object.assign(firewall, fw)
    firewallRules.value = fw.status || fw.default_ruleset || ''
    await Promise.all([loadSessions(), checkUpdates(false)])
    if (auth.pro) await loadVaults()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function loadSessions() {
  try {
    sessions.value = await api.auth.sessions()
  } catch (error) {
    sessions.value = []
  }
}

async function loadVaults() {
  try {
    vaultInfo.value = await api.settings.vaults()
  } catch (error) {
    vaultInfo.value = { vaults: [], available: false }
  }
}

async function saveGeneral() {
  busy.value = true
  try {
    const payload = await api.settings.put('system', { name: general.name, website: general.website })
    settings.value = payload.settings
    ElMessage.success(t('settings.general.saved'))
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

async function changePassword() {
  if (!passwordForm.old || !passwordForm.next) return
  if (passwordForm.next !== passwordForm.confirm) {
    ElMessage.error(t('settings.account.passwordMismatch'))
    return
  }
  if (passwordForm.next.length < 8) {
    ElMessage.error(t('settings.account.weakPassword'))
    return
  }
  busy.value = true
  try {
    await api.auth.changePassword({ old_password: passwordForm.old, new_password: passwordForm.next })
    ElMessage.success(t('settings.account.passwordChanged'))
    passwordForm.old = ''
    passwordForm.next = ''
    passwordForm.confirm = ''
    await loadSessions()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

async function setupTotp() {
  busy.value = true
  try {
    const data = await api.auth.totpSetup()
    totpSetupData.value = data
    qrCode.value = await QRCode.toDataURL(data.otpauth_url, { margin: 1, width: 200, color: { dark: '#0b0f14', light: '#ffffff' } })
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

async function enableTotp() {
  if (!/^\d{6}$/.test(totpCode.value)) {
    ElMessage.warning(t('settings.account.verifyCode'))
    return
  }
  busy.value = true
  try {
    await api.auth.totpEnable(totpCode.value)
    ElMessage.success(t('settings.account.twoFactorEnabled'))
    await auth.fetchMe()
    totpSetupData.value = null
    qrCode.value = ''
    totpCode.value = ''
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

async function disableTotp() {
  busy.value = true
  try {
    await api.auth.totpDisable({ password: disableForm.password, code: disableForm.code })
    ElMessage.success(t('settings.account.twoFactorDisabled'))
    await auth.fetchMe()
    disableForm.password = ''
    disableForm.code = ''
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

async function revokeSession(row) {
  try {
    await ElMessageBox.confirm(t('settings.account.revokeConfirm'), t('common.confirm'), {
      type: 'warning', confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel')
    })
    await api.auth.revokeSession(row.id)
    ElMessage.success(t('common.operationSuccess'))
    await loadSessions()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(errorMessage(error))
  }
}

async function saveSecurity() {
  busy.value = true
  try {
    const payload = await api.settings.put('security', { ...security })
    settings.value = payload.settings
    ElMessage.success(t('settings.security.saved'))
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

async function generateCert() {
  busy.value = true
  try {
    await api.settings.selfSigned(certDomain.value)
    ElMessage.success(t('settings.security.generateCert'))
    Object.assign(ssl, await api.settings.ssl())
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

async function applyFirewall() {
  try {
    await ElMessageBox.prompt(t('network.confirmApply'), t('settings.security.firewallTitle'), {
      type: 'warning', confirmButtonText: t('common.apply'), cancelButtonText: t('common.cancel')
    }).then(async ({ value }) => {
      if (value !== 'APPLY') throw new Error(t('network.confirmApply'))
      busy.value = true
      const result = await api.settings.setFirewall({ ruleset: firewallRules.value, confirm: 'APPLY' })
      ElMessage.success(result.message || t('settings.security.firewallApplied'))
      firewallPending.value = true
      Object.assign(firewall, await api.settings.firewall())
    })
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

async function confirmFirewall() {
  try {
    await api.settings.confirmFirewall()
    firewallPending.value = false
    ElMessage.success(t('common.operationSuccess'))
    Object.assign(firewall, await api.settings.firewall())
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

async function saveNotifications() {
  busy.value = true
  try {
    const payload = await api.settings.put('notifications', { ...notifications })
    settings.value = payload.settings
    ElMessage.success(t('settings.notifications.saved'))
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

async function testNotification() {
  busy.value = true
  try {
    testResults.value = await api.settings.testNotification()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

function downloadBackup() {
  window.open(apiURL('/system/backup'), '_blank')
}

function onRestorePicked(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file) return
  ElMessageBox.prompt(t('settings.backup.restoreConfirm'), t('settings.backup.restoreBackup'), {
    type: 'warning',
    confirmButtonText: t('common.confirm'),
    cancelButtonText: t('common.cancel')
  })
    .then(async ({ value }) => {
      if (value !== 'RESTORE') throw new Error(t('settings.backup.restoreConfirm'))
      const form = new FormData()
      form.append('confirm', 'RESTORE')
      form.append('file', file, file.name)
      busy.value = true
      await api.settings.restoreBackup(form)
      ElMessage.success(t('settings.backup.restoreDone'))
    })
    .catch((error) => {
      if (error !== 'cancel' && error !== 'close') ElMessage.error(errorMessage(error))
    })
    .finally(() => {
      busy.value = false
    })
}

async function saveBackup() {
  busy.value = true
  try {
    const payload = await api.settings.put('backup', { ...backup })
    settings.value = payload.settings
    ElMessage.success(t('settings.backup.saved'))
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

async function saveDestroyPolicy() {
  busy.value = true
  try {
    await api.settings.setDestroyPolicy({ ...destroy })
    ElMessage.success(t('settings.destroy.policySaved'))
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

function openVaultCreate(row) {
  vaultUser.value = row.username
  vaultSize.value = 50
  vaultDialog.value = true
}

async function createVault() {
  busy.value = true
  try {
    await api.settings.createVault(vaultUser.value, vaultSize.value)
    ElMessage.success(t('settings.destroy.vaultCreated'))
    vaultDialog.value = false
    await loadVaults()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

async function vaultAction(action, row) {
  try {
    if (action === 'unlock') await api.settings.unlockVault(row.username)
    else await api.settings.lockVault(row.username)
    ElMessage.success(action === 'unlock' ? t('settings.destroy.vaultUnlocked') : t('settings.destroy.vaultLocked'))
    await loadVaults()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

async function manualDestroy() {
  if (!destroyUsername.value) return
  try {
    const { value } = await ElMessageBox.prompt(t('settings.destroy.confirmUsername'), t('settings.destroy.manualDestroy'), {
      type: 'error',
      confirmButtonText: t('settings.destroy.manualDestroy'),
      cancelButtonText: t('common.cancel')
    })
    if (value !== destroyUsername.value) throw new Error(t('settings.destroy.confirmUsername'))
    busy.value = true
    await api.settings.destroy({ username: destroyUsername.value, confirm: value })
    ElMessage.success(t('settings.destroy.destroyStarted'))
    destroyUsername.value = ''
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

async function checkUpdates(showMessage = true) {
  checkingUpdates.value = true
  try {
    updates.value = await api.settings.updates()
    updatesChecked.value = true
    if (showMessage && !updates.value.length) ElMessage.success(t('settings.general.upToDate'))
  } catch (error) {
    updates.value = []
    if (showMessage) ElMessage.error(errorMessage(error))
  } finally {
    checkingUpdates.value = false
  }
}

async function applyUpdates() {
  applyingUpdates.value = true
  applyOpen.value = true
  applyProgress.value = 0
  applyMessage.value = t('settings.general.updating')
  applyStatus.value = 'running'
  try {
    const job = await api.settings.applyUpdates()
    for (let i = 0; i < 3600; i += 1) {
      await new Promise((resolve) => setTimeout(resolve, 1000))
      const jobs = await api.jobs.list()
      const current = jobs.find((j) => j.id === job.id)
      if (current) {
        applyProgress.value = current.progress || 0
        applyMessage.value = current.message || applyMessage.value
        applyStatus.value = current.status
        if (current.status !== 'running') {
          if (current.status === 'failed') ElMessage.error(current.error || t('common.operationFailed'))
          else ElMessage.success(t('settings.general.updates'))
          setTimeout(() => { applyOpen.value = false }, 800)
          break
        }
      }
    }
    await checkUpdates(false)
  } catch (error) {
    ElMessage.error(errorMessage(error))
    applyOpen.value = false
  } finally {
    applyingUpdates.value = false
  }
}

onMounted(loadAll)
</script>

<style scoped>
.settings-layout {
  display: grid;
  grid-template-columns: 200px minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}
.side-nav {
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 3px;
  position: sticky;
  top: 84px;
}
.side-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border: none;
  background: transparent;
  border-radius: 9px;
  color: var(--pos-muted);
  font-size: 13px;
  cursor: pointer;
  text-align: left;
}
.side-item:hover { background: rgba(139, 161, 180, 0.08); color: var(--pos-text); }
.side-item.active { background: var(--pos-accent-soft); color: var(--pos-accent); }
.main-panel {
  padding: 22px 26px 28px;
  min-height: 60vh;
}
.section { display: flex; flex-direction: column; gap: 16px; }
.section > h3 { margin: 0 0 4px; font-size: 18px; }
.sub-card {
  border: 1px solid var(--pos-border);
  background: var(--pos-panel-2);
  border-radius: 12px;
  padding: 16px 18px;
}
.sub-card h4 { margin: 0 0 12px; font-size: 13.5px; letter-spacing: 0.3px; }
.sub-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-bottom: 12px; }
.sub-head h4 { margin: 0; }
.grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 0 16px; }
.row { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.update-box { display: flex; flex-direction: column; gap: 10px; }
.totp-setup { display: flex; gap: 16px; align-items: flex-start; margin-top: 10px; flex-wrap: wrap; }
.qr { width: 180px; height: 180px; border-radius: 10px; background: #fff; padding: 8px; }
.totp-info { flex: 1; min-width: 180px; display: flex; flex-direction: column; gap: 6px; }
.secret {
  background: rgba(139, 161, 180, 0.1);
  padding: 7px 9px;
  border-radius: 7px;
  word-break: break-all;
}
.cert-path { margin-top: 10px; color: var(--pos-muted); }
.caps {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 10px;
}
.cap-tag { text-transform: uppercase; letter-spacing: 0.4px; }
.info-desc :deep(.el-descriptions__label) {
  width: 96px;
  min-width: 96px;
  white-space: nowrap;
}
.mono-input :deep(textarea) { font-family: 'SF Mono', Menlo, Consolas, monospace; font-size: 12px; line-height: 1.6; }
.test-results { display: flex; flex-direction: column; gap: 6px; }
.test-row { display: flex; align-items: center; gap: 10px; }
.danger-zone { border-color: rgba(248, 113, 113, 0.35); background: rgba(248, 113, 113, 0.05); }
.about { align-items: center; text-align: center; }
.about-logo { width: 74px; height: 74px; }
.about h2 { margin: 0; letter-spacing: 1px; }
.about-table { max-width: 620px; text-align: left; margin: 10px 0; }
.slogan { color: var(--pos-accent); font-size: 13px; margin: 6px 0 0; }
@media (max-width: 960px) {
  .settings-layout { grid-template-columns: 1fr; }
  .side-nav { flex-direction: row; overflow-x: auto; position: static; }
  .side-item { white-space: nowrap; }
  .grid-2, .form-grid { grid-template-columns: 1fr; }
  .main-panel { padding: 16px; }
}
</style>
