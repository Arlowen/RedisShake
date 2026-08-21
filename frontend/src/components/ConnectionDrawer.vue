<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { message } from 'ant-design-vue'
import { PhFlask, PhFloppyDisk, PhShieldCheck } from '@phosphor-icons/vue'

import { api } from '@/api/client'
import type { Connection, ConnectionInput, ConnectionTestResult, TestPurpose } from '@/api/types'
import CheckResultPanel from '@/components/CheckResultPanel.vue'
import { defaultConnectionInput } from '@/utils/defaults'

const props = defineProps<{ open: boolean; connection?: Connection }>()
const emit = defineEmits<{ close: []; saved: [connection: Connection] }>()

const form = reactive<ConnectionInput>(defaultConnectionInput())
const saving = ref(false)
const testing = ref(false)
const testPurpose = ref<TestPurpose>('source')
const testResult = ref<ConnectionTestResult>()
const editing = computed(() => Boolean(props.connection))
const plainForm = () => JSON.parse(JSON.stringify(form)) as ConnectionInput

watch(() => [props.open, props.connection] as const, () => {
  if (!props.open) return
  Object.assign(form, defaultConnectionInput())
  if (props.connection) {
    Object.assign(form, {
      name: props.connection.name,
      topology: props.connection.topology,
      address: props.connection.address ?? '',
      username: props.connection.username ?? '',
      tls: {
        ...defaultConnectionInput().tls,
        enabled: props.connection.tls.enabled,
        server_name: props.connection.tls.server_name ?? '',
        insecure_skip_verify: props.connection.tls.insecure_skip_verify,
      },
      sentinel: {
        ...defaultConnectionInput().sentinel,
        address: props.connection.sentinel.address ?? '',
        master_name: props.connection.sentinel.master_name ?? '',
        username: props.connection.sentinel.username ?? '',
        tls: {
          ...defaultConnectionInput().sentinel.tls,
          enabled: props.connection.sentinel.tls.enabled,
          server_name: props.connection.sentinel.tls.server_name ?? '',
          insecure_skip_verify: props.connection.sentinel.tls.insecure_skip_verify,
        },
      },
    })
  }
  testResult.value = undefined
}, { immediate: true })

function validate() {
  if (!form.name.trim()) throw new Error('请输入连接名称')
  if (form.topology === 'sentinel') {
    if (!form.sentinel.address.trim()) throw new Error('请输入 Sentinel 地址')
    if (!form.sentinel.master_name.trim()) throw new Error('请输入 Master name')
  } else if (!form.address.trim()) throw new Error('请输入 Redis 地址')
}

async function runTest() {
  try {
    validate()
    testing.value = true
    testResult.value = editing.value && props.connection
      ? await api.testSavedConnection(props.connection.id, testPurpose.value)
      : await api.testConnection(plainForm(), testPurpose.value)
    if (testResult.value.success) message.success('连接测试通过')
    else message.warning('连接测试存在阻断项')
  } catch (cause) {
    message.error(cause instanceof Error ? cause.message : '连接测试失败')
  } finally {
    testing.value = false
  }
}

function compactTLS(input: ConnectionInput['tls']) {
  const result: Record<string, unknown> = {
    enabled: input.enabled,
    server_name: input.server_name,
    insecure_skip_verify: input.insecure_skip_verify,
  }
  if (input.ca_cert_pem) result.ca_cert_pem = input.ca_cert_pem
  if (input.client_cert_pem) result.client_cert_pem = input.client_cert_pem
  if (input.client_key_pem) result.client_key_pem = input.client_key_pem
  return result
}

async function save() {
  try {
    validate()
    saving.value = true
    let saved: Connection
    if (props.connection) {
      const patch: Record<string, unknown> = {
        name: form.name,
        topology: form.topology,
        address: form.address,
        username: form.username,
        tls: compactTLS(form.tls),
        sentinel: {
          address: form.sentinel.address,
          master_name: form.sentinel.master_name,
          username: form.sentinel.username,
          tls: compactTLS(form.sentinel.tls),
          ...(form.sentinel.password ? { password: form.sentinel.password } : {}),
        },
      }
      if (form.password) patch.password = form.password
      saved = await api.updateConnection(props.connection.id, patch as Partial<ConnectionInput>)
    } else {
      saved = await api.createConnection(plainForm())
    }
    emit('saved', saved)
    message.success(editing.value ? '连接已更新' : '连接已创建')
  } catch (cause) {
    message.error(cause instanceof Error ? cause.message : '保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <a-drawer :open="open" :width="680" :closable="false" root-class-name="console-drawer" @close="emit('close')">
    <template #title>
      <div class="drawer-title"><span>{{ editing ? '编辑 Redis 连接' : '新建 Redis 连接' }}</span><small>凭据会在控制面加密存储，页面不会再次回显。</small></div>
    </template>
    <div class="connection-form">
      <section class="form-section">
        <div class="form-section-title"><span>基础连接</span><small>RedisShake 用这些信息连接实际数据节点</small></div>
        <div class="form-grid two">
          <label><span>连接名称</span><a-input v-model:value="form.name" placeholder="例如：生产源端" /></label>
          <label><span>拓扑类型</span><a-select v-model:value="form.topology" :options="[{value:'standalone',label:'单机 / 主从'},{value:'sentinel',label:'Sentinel'},{value:'cluster',label:'Cluster'}]" /></label>
        </div>
        <label v-if="form.topology !== 'sentinel'"><span>Redis 地址</span><a-input v-model:value="form.address" placeholder="host:port" /></label>
        <div class="form-grid two">
          <label><span>Redis 用户名</span><a-input v-model:value="form.username" autocomplete="off" placeholder="未启用 ACL 可留空" /></label>
          <label><span>Redis 密码</span><a-input-password v-model:value="form.password" autocomplete="new-password" :placeholder="editing ? '留空表示保持不变' : '未设置密码可留空'" /></label>
        </div>
      </section>

      <section v-if="form.topology === 'sentinel'" class="form-section">
        <div class="form-section-title"><span>Sentinel 发现</span><small>Sentinel 凭据和 Redis 主节点凭据彼此独立</small></div>
        <div class="form-grid two">
          <label><span>Sentinel 地址</span><a-input v-model:value="form.sentinel.address" placeholder="host:26379" /></label>
          <label><span>Master name</span><a-input v-model:value="form.sentinel.master_name" placeholder="mymaster" /></label>
          <label><span>Sentinel 用户名</span><a-input v-model:value="form.sentinel.username" /></label>
          <label><span>Sentinel 密码</span><a-input-password v-model:value="form.sentinel.password" :placeholder="editing ? '留空表示保持不变' : ''" /></label>
        </div>
      </section>

      <section class="form-section">
        <div class="form-section-title"><span>TLS</span><small>默认校验证书；关闭校验仅适合受控测试环境</small></div>
        <div class="tls-switch"><PhShieldCheck :size="22" /><span>Redis TLS</span><a-switch v-model:checked="form.tls.enabled" /></div>
        <template v-if="form.tls.enabled">
          <div class="form-grid two">
            <label><span>Server name</span><a-input v-model:value="form.tls.server_name" placeholder="redis.internal" /></label>
            <label class="switch-field"><span>跳过证书校验</span><a-switch v-model:checked="form.tls.insecure_skip_verify" /></label>
          </div>
          <a-collapse ghost>
            <a-collapse-panel key="certs" header="证书材料（可选）">
              <label><span>CA certificate PEM</span><a-textarea v-model:value="form.tls.ca_cert_pem" :rows="3" /></label>
              <div class="form-grid two">
                <label><span>Client certificate PEM</span><a-textarea v-model:value="form.tls.client_cert_pem" :rows="3" /></label>
                <label><span>Client private key PEM</span><a-textarea v-model:value="form.tls.client_key_pem" :rows="3" /></label>
              </div>
            </a-collapse-panel>
          </a-collapse>
        </template>
      </section>

      <section class="test-section">
        <div class="test-bar">
          <a-segmented v-model:value="testPurpose" :options="[{label:'源端检查',value:'source'},{label:'目标写检查',value:'target'}]" />
          <a-button :loading="testing" @click="runTest"><template #icon><PhFlask :size="17" /></template>测试连接</a-button>
        </div>
        <p v-if="testPurpose === 'target'" class="side-effect-note">目标检查会写入一个带 60 秒 TTL 的随机 Key，并立即尝试删除。</p>
        <CheckResultPanel v-if="testResult" :checks="testResult.checks" title="检查结果" />
      </section>
    </div>
    <template #footer>
      <div class="drawer-footer"><a-button @click="emit('close')">取消</a-button><a-button type="primary" :loading="saving" @click="save"><template #icon><PhFloppyDisk :size="17" /></template>保存连接</a-button></div>
    </template>
  </a-drawer>
</template>

<style scoped>
.drawer-title span,.drawer-title small{display:block}.drawer-title small{margin-top:3px;color:var(--muted);font-size:11px;font-weight:400}
.connection-form{display:grid;gap:18px}.form-section{display:grid;gap:14px;padding:18px;border:1px solid var(--line);border-radius:13px;background:#fff}.form-section-title{display:flex;justify-content:space-between;gap:20px;padding-bottom:10px;border-bottom:1px solid #edf1ef}.form-section-title span{font-weight:650}.form-section-title small{color:var(--muted)}
.form-grid{display:grid;gap:13px}.form-grid.two{grid-template-columns:1fr 1fr}label{display:grid;gap:7px;color:#52605b;font-size:12px}.switch-field{align-content:center;grid-template-columns:1fr auto}.tls-switch{display:grid;grid-template-columns:auto 1fr auto;align-items:center;gap:10px;color:var(--accent)}
.test-section{display:grid;gap:12px}.test-bar,.drawer-footer{display:flex;justify-content:space-between;align-items:center;gap:10px}.side-effect-note{margin:0;color:var(--warning);font-size:11px}
@media(max-width:700px){.form-grid.two{grid-template-columns:1fr}.test-bar{align-items:stretch;flex-direction:column}}
</style>
