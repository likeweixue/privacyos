<template>
  <div class="install-page">
    <header class="install-top">
      <div class="install-brand">
        <img src="/logo.svg" alt="" />
        <strong>PrivacyOS</strong>
        <span>INSTALLER</span>
      </div>
      <div class="install-top-right">
        <span class="preview-chip">{{ t('installer.preview') }}</span>
        <button type="button" class="top-link" @click="ui.toggleLang">{{ ui.lang === 'zh-CN' ? 'English' : '中文' }}</button>
        <router-link class="top-link" :to="{ name: 'login' }">{{ t('installer.backLogin') }}</router-link>
      </div>
    </header>

    <main class="install-main">
      <header class="install-heading">
        <h1>{{ t('installer.title') }}</h1>
        <p>{{ t('installer.subtitle') }}</p>
      </header>

      <div class="install-layout">
        <aside class="install-steps" aria-label="Installer steps">
          <button
            v-for="(label, index) in stepLabels"
            :key="label"
            type="button"
            class="install-step"
            :class="{ active: step === index, done: index < step, disabled: index > maxStep || installing }"
            :aria-current="step === index ? 'step' : undefined"
            @click="goStep(index)"
          >
            <span class="step-number">
              <el-icon v-if="index < step" :size="11"><Check /></el-icon>
              <template v-else>{{ index + 1 }}</template>
            </span>
            <span>{{ label }}</span>
          </button>
        </aside>

        <section class="install-panel">
          <header class="panel-head">
            <div>
              <h2>{{ stepTitle }}</h2>
              <p>{{ stepDesc }}</p>
            </div>
            <el-tag effect="plain" size="small">{{ t('installer.preview') }}</el-tag>
          </header>

          <div class="panel-body">
            <div v-if="error" class="notice danger install-error">
              <el-icon><WarningFilled /></el-icon><span>{{ error }}</span>
            </div>
            <!-- Welcome -->
            <template v-if="step === 0">
              <div class="welcome">
                <img src="/logo.svg" alt="" class="welcome-logo" />
                <h3>{{ t('installer.welcomeTitle') }}</h3>
                <p>{{ t('installer.welcomeDesc') }}</p>
                <ul class="feature-list">
                  <li><el-icon><CircleCheck /></el-icon>{{ t('installer.feature1') }}</li>
                  <li><el-icon><CircleCheck /></el-icon>{{ t('installer.feature2') }}</li>
                  <li><el-icon><CircleCheck /></el-icon>{{ t('installer.feature3') }}</li>
                </ul>
              </div>
            </template>

            <!-- Edition -->
            <template v-else-if="step === 1">
              <div class="choice-grid">
                <button
                  type="button"
                  class="choice-card"
                  :class="{ selected: form.edition === 'normal' }"
                  @click="form.edition = 'normal'"
                >
                  <span class="choice-icon"><el-icon :size="22"><HomeFilled /></el-icon></span>
                  <strong>{{ t('installer.normal') }}</strong>
                  <p>{{ t('installer.normalDesc') }}</p>
                  <el-tag v-if="form.edition === 'normal'" size="small" type="success" effect="plain">{{ t('installer.enabled') }}</el-tag>
                </button>
                <button
                  type="button"
                  class="choice-card"
                  :class="{ selected: form.edition === 'pro' }"
                  @click="form.edition = 'pro'"
                >
                  <span class="choice-icon pro"><el-icon :size="22"><Lock /></el-icon></span>
                  <strong>{{ t('installer.pro') }}</strong>
                  <p>{{ t('installer.proDesc') }}</p>
                  <el-tag v-if="form.edition === 'pro'" size="small" type="warning" effect="plain">{{ t('installer.enabled') }}</el-tag>
                </button>
              </div>
              <div v-if="form.edition === 'pro'" class="notice warning">
                <el-icon><WarningFilled /></el-icon>
                <span>{{ t('installer.proNote') }}</span>
              </div>
            </template>

            <!-- Disk -->
            <template v-else-if="step === 2">
              <div class="disk-grid">
                <button
                  v-for="disk in disks"
                  :key="disk.path"
                  type="button"
                  class="disk-card"
                  :class="{ selected: form.disk === disk.path }"
                  @click="form.disk = disk.path"
                >
                  <span class="disk-icon"><el-icon :size="20"><Coin /></el-icon></span>
                  <span class="disk-info">
                    <strong>{{ disk.path }}</strong>
                    <small>{{ disk.model }} · {{ disk.size }}</small>
                  </span>
                  <span class="disk-type">{{ disk.type }}</span>
                </button>
              </div>
              <div class="form-grid">
                <label class="field">
                  <span>{{ t('installer.hostname') }}</span>
                  <el-input v-model="form.hostname" placeholder="privacyos" />
                </label>
                <label class="field">
                  <span>{{ t('installer.timezone') }}</span>
                  <el-select v-model="form.timezone" style="width: 100%">
                    <el-option label="Asia/Shanghai" value="Asia/Shanghai" />
                    <el-option label="Asia/Tokyo" value="Asia/Tokyo" />
                    <el-option label="Europe/Berlin" value="Europe/Berlin" />
                    <el-option label="America/New_York" value="America/New_York" />
                    <el-option label="UTC" value="UTC" />
                  </el-select>
                </label>
              </div>
              <div class="notice danger">
                <el-icon><WarningFilled /></el-icon>
                <span>{{ t('installer.diskWarning') }}</span>
              </div>
            </template>

            <!-- Account -->
            <template v-else-if="step === 3">
              <div class="form-grid">
                <label class="field">
                  <span>{{ t('installer.username') }}</span>
                  <el-input v-model="form.username" placeholder="padmin" />
                </label>
                <label class="field">
                  <span>{{ t('installer.password') }}</span>
                  <el-input v-model="form.password" type="password" show-password autocomplete="new-password" />
                </label>
                <label class="field">
                  <span>{{ t('installer.confirm') }}</span>
                  <el-input v-model="form.passwordConfirm" type="password" show-password autocomplete="new-password" />
                </label>
                <div v-if="form.edition === 'pro'" class="field">
                  <span>{{ t('installer.luks') }}</span>
                  <el-input v-model="form.luks" type="password" show-password autocomplete="new-password" />
                </div>
              </div>
              <div class="password-meter">
                <span>{{ t('installer.passwordHint') }}</span>
                <div class="meter-track"><i :style="{ width: `${passwordStrength * 33}%`, background: strengthColor }" /></div>
              </div>
              <div v-if="form.edition === 'pro'" class="notice warning">
                <el-icon><Lock /></el-icon>
                <span>{{ t('installer.luksHint') }}</span>
              </div>
            </template>

            <!-- Network -->
            <template v-else-if="step === 4">
              <el-radio-group v-model="form.network" class="network-choice">
                <el-radio-button value="dhcp">{{ t('installer.dhcp') }}</el-radio-button>
                <el-radio-button value="static">{{ t('installer.static') }}</el-radio-button>
              </el-radio-group>
              <div v-if="form.network === 'static'" class="form-grid">
                <label class="field">
                  <span>{{ t('installer.ip') }}</span>
                  <el-input v-model="form.ip" placeholder="192.168.1.10/24" />
                </label>
                <label class="field">
                  <span>{{ t('installer.gateway') }}</span>
                  <el-input v-model="form.gateway" placeholder="192.168.1.1" />
                </label>
                <label class="field">
                  <span>{{ t('installer.dns') }}</span>
                  <el-input v-model="form.dns" placeholder="1.1.1.1" />
                </label>
              </div>
            </template>

            <!-- Summary -->
            <template v-else-if="step === 5">
              <div class="summary">
                <div class="summary-row"><span>{{ t('installer.editionLabel') }}</span><strong>{{ form.edition === 'pro' ? t('installer.pro') : t('installer.normal') }}</strong></div>
                <div class="summary-row"><span>{{ t('installer.diskLabel') }}</span><strong>{{ form.disk }} · {{ selectedDisk?.size }}</strong></div>
                <div class="summary-row"><span>{{ t('installer.hostnameLabel') }}</span><strong>{{ form.hostname }}</strong></div>
                <div class="summary-row"><span>{{ t('installer.adminLabel') }}</span><strong>{{ form.username }}</strong></div>
                <div class="summary-row"><span>{{ t('installer.networkLabel') }}</span><strong>{{ form.network === 'static' ? `${form.ip} · ${form.gateway}` : t('installer.dhcp') }}</strong></div>
                <div class="summary-row"><span>{{ t('installer.securityLabel') }}</span><strong>{{ form.edition === 'pro' ? t('installer.enabled') : t('installer.disabled') }}</strong></div>
              </div>
              <div class="notice danger">
                <el-icon><WarningFilled /></el-icon>
                <span>{{ t('installer.diskWarning') }}</span>
              </div>
              <p class="pos-hint">{{ t('installer.summaryHint') }}</p>
            </template>

            <!-- Progress / done -->
            <template v-else>
              <div v-if="!done" class="progress-box">
                <h3>{{ t('installer.installTitle') }}</h3>
                <p>{{ t('installer.installDesc') }}</p>
                <el-progress :percentage="progress" :stroke-width="6" :show-text="true" />
                <ul class="phase-list">
                  <li v-for="(phase, index) in phases" :key="phase" :class="phaseClass(index)">
                    <el-icon v-if="index < phaseIndex"><CircleCheck /></el-icon>
                    <el-icon v-else-if="index === phaseIndex" class="is-loading"><Loading /></el-icon>
                    <span v-else class="pending-dot" />
                    <span>{{ phase }}</span>
                  </li>
                </ul>
              </div>
              <div v-else class="done-box">
                <el-icon :size="44" class="done-icon"><CircleCheck /></el-icon>
                <h3>{{ t('installer.doneTitle') }}</h3>
                <p>{{ t('installer.doneDesc') }}</p>
                <div class="done-card">
                  <div><span>{{ t('installer.adminUrl') }}</span><strong>http://{{ form.hostname }}.local</strong></div>
                  <div><span>{{ t('installer.adminLabel') }}</span><strong>{{ form.username }}</strong></div>
                </div>
                <div class="done-actions">
                  <el-button :icon="Download" @click="downloadPreseed">{{ t('installer.downloadPreseed') }}</el-button>
                  <el-button type="primary" @click="$router.push({ name: 'login' })">{{ t('installer.backLogin') }}</el-button>
                </div>
              </div>
            </template>
          </div>

          <footer v-if="step < 6" class="panel-foot">
            <el-button v-if="step > 0" :icon="ArrowLeft" @click="prevStep">{{ t('installer.back') }}</el-button>
            <div class="spacer" />
            <el-button v-if="step < 5" type="primary" @click="nextStep">
              {{ step === 0 ? t('installer.begin') : t('installer.next') }}<el-icon class="el-icon--right"><ArrowRight /></el-icon>
            </el-button>
            <el-button v-else type="primary" @click="startInstall">{{ t('installer.startInstall') }}</el-button>
          </footer>
        </section>
      </div>

      <p class="preview-note">{{ t('installer.previewHint') }}</p>
    </main>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ArrowLeft, ArrowRight, Check, CircleCheck, Coin, Download, HomeFilled,
  Loading, Lock, WarningFilled
} from '@element-plus/icons-vue'
import { useUiStore } from '../stores/ui'

const { t } = useI18n()
const ui = useUiStore()

const step = ref(0)
const maxStep = ref(0)
const installing = ref(false)
const done = ref(false)
const progress = ref(0)
const error = ref('')
let timer = null

const form = reactive({
  edition: 'normal',
  disk: '/dev/nvme0n1',
  hostname: 'privacyos',
  timezone: 'Asia/Shanghai',
  username: 'padmin',
  password: '',
  passwordConfirm: '',
  luks: '',
  network: 'dhcp',
  ip: '192.168.1.10/24',
  gateway: '192.168.1.1',
  dns: '1.1.1.1'
})

const disks = [
  { path: '/dev/nvme0n1', model: 'Samsung SSD 990 PRO', size: '1 TB', type: 'NVMe' },
  { path: '/dev/sda', model: 'WDC WD40EFPX', size: '4 TB', type: 'HDD' },
  { path: '/dev/vda', model: 'QEMU Virtual Disk', size: '80 GB', type: 'VirtIO' }
]

const stepLabels = computed(() => [
  t('installer.steps.welcome'), t('installer.steps.edition'), t('installer.steps.disk'),
  t('installer.steps.account'), t('installer.steps.network'), t('installer.steps.summary'),
  t('installer.steps.install')
])
const titles = computed(() => [
  t('installer.welcomeTitle'), t('installer.editionTitle'), t('installer.diskTitle'),
  t('installer.accountTitle'), t('installer.networkTitle'), t('installer.summaryTitle'),
  t('installer.installTitle')
])
const descriptions = computed(() => [
  t('installer.welcomeDesc'), t('installer.editionDesc'), t('installer.diskDesc'),
  t('installer.accountDesc'), t('installer.networkDesc'), t('installer.summaryDesc'),
  t('installer.installDesc')
])
const stepTitle = computed(() => (step.value === 6 && done.value ? t('installer.doneTitle') : titles.value[step.value]))
const stepDesc = computed(() => (step.value === 6 && done.value ? t('installer.doneDesc') : descriptions.value[step.value]))
const selectedDisk = computed(() => disks.find((d) => d.path === form.disk))
const phases = computed(() => [
  t('installer.phase1'), t('installer.phase2'), t('installer.phase3'),
  t('installer.phase4'), t('installer.phase5')
])
const phaseIndex = computed(() => {
  if (progress.value >= 96) return 4
  if (progress.value >= 72) return 3
  if (progress.value >= 46) return 2
  if (progress.value >= 20) return 1
  return 0
})
const passwordStrength = computed(() => {
  const p = form.password
  let score = 0
  if (p.length >= 8) score++
  if (/[A-Z]/.test(p) && /[a-z]/.test(p)) score++
  if (/\d/.test(p) && /[^A-Za-z0-9]/.test(p)) score++
  return score
})
const strengthColor = computed(() => ['#d64545', '#b7791f', '#0d9488', '#2f855a'][passwordStrength.value] || '#d64545')

function goStep(index) {
  if (installing.value || done.value) return
  if (index <= maxStep.value) {
    step.value = index
    error.value = ''
  }
}

function validate() {
  error.value = ''
  if (step.value === 2) {
    if (!form.disk || !form.hostname.trim()) { error.value = t('installer.required'); return false }
  }
  if (step.value === 3) {
    if (!form.username.trim() || !form.password || !form.passwordConfirm) { error.value = t('installer.required'); return false }
    if (form.password.length < 8) { error.value = t('installer.weakPassword'); return false }
    if (form.password !== form.passwordConfirm) { error.value = t('installer.mismatch'); return false }
    if (form.edition === 'pro' && form.luks.length < 12) { error.value = t('installer.weakLuks'); return false }
  }
  if (step.value === 4 && form.network === 'static') {
    if (!form.ip.trim() || !form.gateway.trim()) { error.value = t('installer.required'); return false }
  }
  return true
}

function nextStep() {
  if (!validate()) return
  if (step.value < 5) {
    step.value++
    maxStep.value = Math.max(maxStep.value, step.value)
  }
}
function prevStep() {
  if (step.value > 0) step.value--
}
function startInstall() {
  if (!validate()) return
  step.value = 6
  maxStep.value = 6
  installing.value = true
  progress.value = 0
  done.value = false
  timer = setInterval(() => {
    progress.value = Math.min(100, progress.value + 2)
    if (progress.value >= 100) {
      clearInterval(timer)
      timer = null
      installing.value = false
      done.value = true
    }
  }, 140)
}

function phaseClass(index) {
  return {
    done: index < phaseIndex.value || done.value,
    active: index === phaseIndex.value && !done.value
  }
}

function downloadPreseed() {
  const line = (cmd, question, value) => `${cmd} ${question} ${value}`
  const lines = [
    '# PrivacyOS preseed — generated by the installer preview',
    `# ${new Date().toISOString()}`,
    '',
    line('d-i', 'debian-installer/locale', 'string zh_CN.UTF-8'),
    line('d-i', 'netcfg/get_hostname', `string ${form.hostname}`),
    line('d-i', 'netcfg/choose_interface', 'select auto'),
    line('d-i', 'time/zone', `string ${form.timezone}`),
    line('d-i', 'passwd/username', `string ${form.username}`),
    line('d-i', 'passwd/user-password', `password ${form.password}`),
    line('d-i', 'passwd/user-password-again', `password ${form.password}`),
    line('d-i', 'partman-auto/disk', `string ${form.disk}`),
    line('d-i', 'partman-auto/method', `string ${form.edition === 'pro' ? 'crypt' : 'regular'}`),
    line('d-i', 'partman-auto/choose_recipe', 'select atomic'),
    line('d-i', 'partman/confirm', 'boolean true'),
    line('d-i', 'partman/confirm_nooverwrite', 'boolean true'),
    line('d-i', 'pkgsel/include', 'string sudo ca-certificates curl openssh-server nftables cryptsetup smartmontools mdadm'),
    line('d-i', 'grub-installer/bootdev', 'string default'),
    line('d-i', 'finish-install/reboot_in_progress', 'note')
  ]
  if (form.edition === 'pro') {
    lines.push(line('d-i', 'partman-crypto/passphrase', `password ${form.luks}`))
    lines.push(line('d-i', 'partman-crypto/passphrase-again', `password ${form.luks}`))
  }
  if (form.network === 'static') {
    lines.push(line('d-i', 'netcfg/disable_autoconfig', 'boolean true'))
    lines.push(line('d-i', 'netcfg/get_ipaddress', `string ${form.ip.split('/')[0]}`))
    lines.push(line('d-i', 'netcfg/get_netmask', 'string 255.255.255.0'))
    lines.push(line('d-i', 'netcfg/get_gateway', `string ${form.gateway}`))
    lines.push(line('d-i', 'netcfg/get_nameservers', `string ${form.dns}`))
  }
  const blob = new Blob([lines.join('\n') + '\n'], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `privacyos-${form.edition}-preseed.cfg`
  a.click()
  URL.revokeObjectURL(url)
}

onBeforeUnmount(() => { if (timer) clearInterval(timer) })
</script>

<style scoped>
.install-page { min-height: 100vh; background: var(--pos-bg); color: var(--pos-text); }
.install-top {
  position: sticky; top: 0; z-index: 20; height: 56px;
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 24px; background: var(--pos-panel); border-bottom: 1px solid var(--pos-border);
}
.install-brand { display: flex; align-items: center; gap: 9px; }
.install-brand img { width: 27px; height: 27px; }
.install-brand strong { font-size: 14.5px; letter-spacing: 0.2px; }
.install-brand span {
  font-size: 9.5px; letter-spacing: 0.9px; color: var(--pos-muted);
  border-left: 1px solid var(--pos-border); padding-left: 8px;
}
.install-top-right { display: flex; align-items: center; gap: 4px; }
.preview-chip {
  font-size: 10px; color: var(--pos-warning); border: 1px solid color-mix(in srgb, var(--pos-warning) 35%, var(--pos-border));
  padding: 2px 7px; margin-right: 6px;
}
.top-link {
  border: 1px solid transparent; background: transparent; color: var(--pos-muted);
  font-size: 11.5px; padding: 6px 8px; cursor: pointer; text-decoration: none; border-radius: 2px;
}
.top-link:hover { background: var(--pos-panel-2); color: var(--pos-text); }
.install-main { max-width: 1180px; margin: 0 auto; padding: 26px 24px 46px; }
.install-heading h1 { margin: 0; font-size: 22px; font-weight: 650; }
.install-heading p { margin: 4px 0 0; font-size: 12.5px; color: var(--pos-muted); }
.install-layout {
  display: grid; grid-template-columns: 210px minmax(0, 1fr);
  margin-top: 16px; border: 1px solid var(--pos-border); background: var(--pos-panel);
}
.install-steps {
  display: flex; flex-direction: column; gap: 2px; padding: 8px;
  background: var(--pos-panel); border-right: 1px solid var(--pos-border);
}
.install-step {
  display: flex; align-items: center; gap: 9px; padding: 9px 10px;
  border: none; border-left: 2px solid transparent; background: transparent;
  color: var(--pos-text-2); font-size: 12.5px; text-align: left; cursor: pointer;
}
.install-step:hover { background: var(--pos-panel-2); }
.install-step.active { background: var(--pos-accent-soft); border-left-color: var(--pos-accent); color: var(--pos-accent); font-weight: 600; }
.install-step.done { color: var(--pos-text); }
.install-step.disabled { opacity: 0.45; cursor: default; }
.step-number {
  width: 20px; height: 20px; flex-shrink: 0; display: inline-flex; align-items: center; justify-content: center;
  border: 1px solid var(--pos-border-strong); border-radius: 50%; font-size: 10px;
}
.install-step.active .step-number { background: var(--pos-accent); border-color: var(--pos-accent); color: #fff; }
.install-step.done .step-number { border-color: var(--pos-success); color: var(--pos-success); }
.install-panel { display: flex; flex-direction: column; min-width: 0; background: var(--pos-bg); }
.panel-head {
  display: flex; align-items: flex-start; justify-content: space-between; gap: 12px;
  padding: 15px 20px; background: var(--pos-panel); border-bottom: 1px solid var(--pos-border);
}
.panel-head h2 { margin: 0; font-size: 16px; font-weight: 650; }
.panel-head p { margin: 3px 0 0; font-size: 11.5px; color: var(--pos-muted); }
.panel-body { flex: 1; display: flex; flex-direction: column; gap: 12px; padding: 18px 20px; min-width: 0; }
.panel-body > * { min-width: 0; }
.install-error { margin: 0; }
.welcome { padding: 24px; background: var(--pos-panel); border: 1px solid var(--pos-border); text-align: center; }
.welcome-logo { width: 54px; height: 54px; }
.welcome h3 { margin: 10px 0 6px; font-size: 18px; }
.welcome p { margin: 0 auto; max-width: 560px; color: var(--pos-muted); font-size: 12.5px; line-height: 1.65; }
.feature-list { list-style: none; padding: 0; margin: 18px auto 0; display: flex; flex-direction: column; gap: 9px; max-width: 420px; text-align: left; }
.feature-list li { display: flex; align-items: center; gap: 8px; font-size: 12px; color: var(--pos-text-2); }
.feature-list .el-icon { color: var(--pos-success); }
.choice-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.choice-card {
  display: flex; flex-direction: column; align-items: flex-start; gap: 6px;
  padding: 16px; background: var(--pos-panel); border: 1px solid var(--pos-border);
  border-radius: 2px; color: var(--pos-text); text-align: left; cursor: pointer;
}
.choice-card:hover { border-color: var(--pos-border-strong); }
.choice-card.selected { border-color: var(--pos-accent); background: var(--pos-accent-soft); }
.choice-icon {
  width: 38px; height: 38px; display: inline-flex; align-items: center; justify-content: center;
  border: 1px solid var(--pos-border); background: var(--pos-panel); color: var(--pos-accent);
}
.choice-icon.pro { color: var(--pos-warning); }
.choice-card strong { font-size: 14px; }
.choice-card p { margin: 0; font-size: 11.5px; line-height: 1.55; color: var(--pos-muted); }
.choice-card .el-tag { margin-top: 2px; }
.notice {
  display: flex; align-items: flex-start; gap: 7px; padding: 9px 11px;
  border: 1px solid var(--pos-border); font-size: 11.5px; line-height: 1.55;
}
.notice.warning { color: var(--pos-warning); border-color: color-mix(in srgb, var(--pos-warning) 30%, var(--pos-border)); background: color-mix(in srgb, var(--pos-warning) 6%, transparent); }
.notice.danger { color: var(--pos-danger); border-color: color-mix(in srgb, var(--pos-danger) 28%, var(--pos-border)); background: color-mix(in srgb, var(--pos-danger) 5%, transparent); }
.disk-grid { display: flex; flex-direction: column; gap: 6px; }
.disk-card {
  display: flex; align-items: center; gap: 10px; width: 100%; padding: 10px 12px;
  border: 1px solid var(--pos-border); background: var(--pos-panel); color: var(--pos-text);
  text-align: left; cursor: pointer;
}
.disk-card:hover { border-color: var(--pos-border-strong); }
.disk-card.selected { border-color: var(--pos-accent); background: var(--pos-accent-soft); }
.disk-icon { width: 30px; height: 30px; display: inline-flex; align-items: center; justify-content: center; border: 1px solid var(--pos-border); color: var(--pos-muted); }
.disk-info { flex: 1; display: flex; flex-direction: column; gap: 1px; }
.disk-info strong { font-size: 12.5px; }
.disk-info small { font-size: 10.5px; color: var(--pos-muted); }
.disk-type { font-size: 9.5px; color: var(--pos-muted); border: 1px solid var(--pos-border); padding: 1px 5px; }
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px 16px; }
.field { display: flex; flex-direction: column; gap: 5px; font-size: 11.5px; color: var(--pos-muted); }
.network-choice { margin: 0 0 4px; }
.password-meter { display: flex; flex-direction: column; gap: 5px; font-size: 10.5px; color: var(--pos-muted); }
.meter-track { height: 4px; background: var(--pos-panel-2); }
.meter-track i { display: block; height: 100%; transition: width 0.2s ease; }
.summary { background: var(--pos-panel); border: 1px solid var(--pos-border); }
.summary-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 10px 14px; border-bottom: 1px solid var(--pos-border); font-size: 12px; }
.summary-row:last-child { border-bottom: none; }
.summary-row span { color: var(--pos-muted); }
.summary-row strong { font-size: 12.5px; font-weight: 550; text-align: right; word-break: break-word; }
.progress-box { padding: 20px; background: var(--pos-panel); border: 1px solid var(--pos-border); }
.progress-box h3 { margin: 0; font-size: 16px; }
.progress-box > p { margin: 4px 0 14px; font-size: 11.5px; color: var(--pos-muted); }
.phase-list { list-style: none; margin: 16px 0 0; padding: 0; display: flex; flex-direction: column; gap: 9px; }
.phase-list li { display: flex; align-items: center; gap: 8px; font-size: 12px; color: var(--pos-muted); }
.phase-list li.active { color: var(--pos-text); font-weight: 550; }
.phase-list li.done { color: var(--pos-success); }
.phase-list .is-loading { animation: spin 1s linear infinite; color: var(--pos-accent); }
@keyframes spin { to { transform: rotate(360deg); } }
.pending-dot { width: 13px; height: 13px; border: 1px solid var(--pos-border-strong); border-radius: 50%; display: inline-block; }
.done-box { padding: 28px; background: var(--pos-panel); border: 1px solid var(--pos-border); text-align: center; }
.done-icon { color: var(--pos-success); }
.done-box h3 { margin: 8px 0 4px; font-size: 18px; }
.done-box > p { margin: 0 auto; max-width: 540px; color: var(--pos-muted); font-size: 12px; line-height: 1.65; }
.done-card { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; max-width: 520px; margin: 16px auto 0; padding: 12px; background: var(--pos-panel-2); border: 1px solid var(--pos-border); text-align: left; }
.done-card div { display: flex; flex-direction: column; gap: 2px; }
.done-card span { font-size: 10px; color: var(--pos-muted); }
.done-card strong { font-size: 12.5px; }
.done-actions { display: flex; justify-content: center; gap: 8px; flex-wrap: wrap; margin-top: 16px; }
.panel-foot { display: flex; align-items: center; gap: 8px; padding: 11px 18px; background: var(--pos-panel); border-top: 1px solid var(--pos-border); }
.panel-foot .spacer { flex: 1; }
.preview-note { margin: 14px 0 0; text-align: center; font-size: 10.5px; color: var(--pos-muted); }
@media (max-width: 900px) {
  .install-top { padding: 0 14px; }
  .install-brand span { display: none; }
  .preview-chip { display: none; }
  .install-main { padding: 18px 14px 34px; }
  .install-layout { grid-template-columns: minmax(0, 1fr); }
  .install-steps { flex-direction: row; overflow-x: auto; border-right: none; border-bottom: 1px solid var(--pos-border); }
  .install-step { flex-shrink: 0; border-left: none; border-bottom: 2px solid transparent; padding: 8px 10px; }
  .install-step.active { border-left-color: transparent; border-bottom-color: var(--pos-accent); }
  .panel-body { padding: 14px; }
  .choice-grid { grid-template-columns: 1fr; }
  .form-grid { grid-template-columns: 1fr; }
  .done-card { grid-template-columns: 1fr; }
}
</style>
