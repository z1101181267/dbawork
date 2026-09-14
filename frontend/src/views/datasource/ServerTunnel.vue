<script setup>
import { ref, watch } from 'vue'

/**
 * 服务器隧道子表单（SSH for Linux / WinRM for Windows）
 * 直接读写父级 form 对象上的 tunnel_* 字段，避免多层 v-model 代理带来的同步问题。
 */
const props = defineProps({
  form: { type: Object, required: true }
})

const form = props.form

// SSH 认证方式（UI 局部状态，不持久化）
const sshAuth = ref(form.tunnel_private_key ? 'key' : 'password')

// 切换隧道类型时补默认值
watch(
  () => form.tunnel_type,
  (v) => {
    if (v === 'ssh') {
      if (!form.tunnel_port || [5985, 5986].includes(Number(form.tunnel_port))) form.tunnel_port = 22
      if (!form.tunnel_private_key) sshAuth.value = 'password'
    } else if (v === 'winrm') {
      if (!form.tunnel_transport) form.tunnel_transport = 'http'
      if (!form.tunnel_auth_scheme) form.tunnel_auth_scheme = 'ntlm'
      if (!form.tunnel_port || Number(form.tunnel_port) === 22) {
        form.tunnel_port = form.tunnel_transport === 'https' ? 5986 : 5985
      }
    }
  }
)

// 切换传输协议时联动端口与 SSL
watch(
  () => form.tunnel_transport,
  (v) => {
    if (form.tunnel_type !== 'winrm' || !v) return
    form.tunnel_use_ssl = v === 'https'
    if (!form.tunnel_port || [5985, 5986].includes(form.tunnel_port)) {
      form.tunnel_port = v === 'https' ? 5986 : 5985
    }
  }
)

// 私钥文件上传：读取文本内容填入
async function onKeyFileChange(uploadFile) {
  const file = uploadFile?.raw
  if (!file) return
  form.tunnel_private_key = await file.text()
  sshAuth.value = 'key'
}
</script>

<template>
  <div class="tunnel-form">
    <el-form-item label="隧道类型">
      <el-radio-group v-model="form.tunnel_type" class="tunnel-type">
        <el-radio-button value="none">不使用</el-radio-button>
        <el-radio-button value="ssh">SSH（Linux）</el-radio-button>
        <el-radio-button value="winrm">WinRM（Windows）</el-radio-button>
      </el-radio-group>
    </el-form-item>

    <!-- SSH -->
    <template v-if="form.tunnel_type === 'ssh'">
      <div class="grid-2">
        <el-form-item label="SSH 主机">
          <el-input v-model="form.tunnel_host" placeholder="跳板机 / 数据库服务器 IP" />
        </el-form-item>
        <el-form-item label="SSH 端口">
          <el-input-number v-model="form.tunnel_port" :min="1" :max="65535" controls-position="right" />
        </el-form-item>
      </div>
      <div class="grid-2">
        <el-form-item label="SSH 用户名">
          <el-input v-model="form.tunnel_user" placeholder="root / dba" />
        </el-form-item>
        <el-form-item label="认证方式">
          <el-radio-group v-model="sshAuth">
            <el-radio-button value="password">密码</el-radio-button>
            <el-radio-button value="key">私钥</el-radio-button>
          </el-radio-group>
        </el-form-item>
      </div>
      <el-form-item v-if="sshAuth === 'password'" label="SSH 密码">
        <el-input v-model="form.tunnel_password" type="password" show-password placeholder="编辑时留空表示保持原值" />
      </el-form-item>
      <template v-else>
        <el-form-item label="私钥内容">
          <el-input
            v-model="form.tunnel_private_key"
            type="textarea"
            :rows="4"
            placeholder="粘贴 PEM 私钥内容（-----BEGIN ...），编辑时留空表示保持原值"
          />
        </el-form-item>
        <div class="grid-2">
          <el-form-item label="私钥口令">
            <el-input v-model="form.tunnel_key_passphrase" type="password" show-password placeholder="无口令可留空" />
          </el-form-item>
          <el-form-item label="或上传私钥文件">
            <el-upload :auto-upload="false" :show-file-list="false" :on-change="onKeyFileChange">
              <el-button plain>选择文件</el-button>
            </el-upload>
          </el-form-item>
        </div>
      </template>
    </template>

    <!-- WinRM -->
    <template v-if="form.tunnel_type === 'winrm'">
      <div class="grid-2">
        <el-form-item label="WinRM 主机">
          <el-input v-model="form.tunnel_host" placeholder="Windows 服务器 IP" />
        </el-form-item>
        <el-form-item label="WinRM 端口">
          <el-input-number v-model="form.tunnel_port" :min="1" :max="65535" controls-position="right" />
        </el-form-item>
      </div>
      <div class="grid-2">
        <el-form-item label="用户名">
          <el-input v-model="form.tunnel_user" placeholder="administrator / DOMAIN\\user" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.tunnel_password" type="password" show-password placeholder="编辑时留空表示保持原值" />
        </el-form-item>
      </div>
      <div class="grid-2">
        <el-form-item label="认证方式">
          <el-select v-model="form.tunnel_auth_scheme" style="width: 100%">
            <el-option label="basic" value="basic" />
            <el-option label="ntlm（推荐）" value="ntlm" />
            <el-option label="kerberos" value="kerberos" />
          </el-select>
        </el-form-item>
        <el-form-item label="传输协议 / SSL">
          <div class="inline">
            <el-radio-group v-model="form.tunnel_transport">
              <el-radio-button value="http">http</el-radio-button>
              <el-radio-button value="https">https</el-radio-button>
            </el-radio-group>
            <el-switch v-model="form.tunnel_use_ssl" active-text="SSL" />
          </div>
        </el-form-item>
      </div>
      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="WinRM 隧道通过远程 netsh portproxy 建立端口转发，需要 Windows 服务器管理员权限且已开启 WinRM。"
      />
    </template>
  </div>
</template>

<style scoped>
.grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 16px;
}

.inline {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* 隧道类型按钮加大，提升可点性 */
.tunnel-type :deep(.el-radio-button__inner) {
  padding: 9px 20px;
  font-size: 14px;
}
</style>
