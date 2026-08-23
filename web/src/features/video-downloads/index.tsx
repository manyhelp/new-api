import { useCallback, useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

interface VideoDownload {
  id: number
  task_id: string
  status: string
  file_size: number
  public_url: string
  error: string
  retry_count: number
  created_at: string
  updated_at: string
}

interface OptionPair {
  key: string
  value: string
}

const PAGE_SIZE = 20
const STATUS_OPTIONS = ['', 'pending', 'downloading', 'success', 'failed']
const SELECT_CLASS =
  'h-8 rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm outline-none transition-colors focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30'

// 设置项默认值（与后端 setting/video_localize_setting.go 保持一致）
const OPT_DEFAULTS: Record<string, string> = {
  VideoLocalizeEnabled: 'true',
  VideoLocalizeConcurrency: '3',
  VideoLocalizeDir: 'data/video_files',
  VideoLocalizeTimeoutSeconds: '120',
  VideoLocalizeMaxRetry: '3',
  VideoLocalizeRetainDays: '0',
  VideoLocalizePublicBaseURL: '',
}
const OPT_LABELS: Record<string, string> = {
  VideoLocalizeEnabled: 'Enable Video Localization',
  VideoLocalizeConcurrency: 'Max Concurrency',
  VideoLocalizeDir: 'Storage Directory',
  VideoLocalizeTimeoutSeconds: 'Download Timeout (s)',
  VideoLocalizeMaxRetry: 'Max Retry',
  VideoLocalizeRetainDays: 'Retain Days (0=no cleanup)',
  VideoLocalizePublicBaseURL: 'Public Base URL (empty = system address)',
}
const OPT_KEYS = Object.keys(OPT_DEFAULTS)

function errMsg(e: unknown, fallback: string): string {
  const err = e as { response?: { data?: { message?: string } }; message?: string }
  return err?.response?.data?.message || err?.message || fallback
}

function formatSize(n: number): string {
  if (!n) return '-'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / (1024 * 1024)).toFixed(2)} MB`
}

export function VideoDownloads() {
  const { t } = useTranslation()

  // 列表状态
  const [list, setList] = useState<VideoDownload[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [status, setStatus] = useState('')
  const [taskId, setTaskId] = useState('')
  const [loading, setLoading] = useState(false)
  const [msg, setMsg] = useState('')

  // 设置状态
  const [opts, setOpts] = useState<Record<string, string>>({ ...OPT_DEFAULTS })
  const [optsDirty, setOptsDirty] = useState(false)
  const [optsMsg, setOptsMsg] = useState('')
  const [optsAllowed, setOptsAllowed] = useState(true)

  const loadList = useCallback(async () => {
    setLoading(true)
    setMsg('')
    try {
      const res = await api.get('/api/video-download/list', {
        params: { page, page_size: PAGE_SIZE, status, task_id: taskId },
      })
      setList(res.data?.data?.list || [])
      setTotal(res.data?.data?.total || 0)
    } catch (e) {
      setMsg(errMsg(e, t('Request failed')))
    } finally {
      setLoading(false)
    }
  }, [page, status, taskId, t])

  const loadOpts = useCallback(async () => {
    try {
      const res = await api.get('/api/option/')
      const arr: OptionPair[] = res.data?.data || []
      const map: Record<string, string> = { ...OPT_DEFAULTS }
      for (const o of arr) {
        if (o.key in OPT_DEFAULTS) map[o.key] = o.value
      }
      setOpts(map)
      setOptsDirty(false)
      setOptsAllowed(true)
    } catch (e) {
      // 非 root 无权读/改全局设置
      setOptsAllowed(false)
    }
  }, [])

  useEffect(() => {
    loadList()
  }, [loadList])
  useEffect(() => {
    loadOpts()
  }, [loadOpts])

  const retry = async (id: string) => {
    try {
      await api.post(`/api/video-download/retry/${id}`)
      await loadList()
    } catch (e) {
      setMsg(errMsg(e, t('Request failed')))
    }
  }

  const remove = async (id: number) => {
    if (!window.confirm(t('Confirm delete?'))) return
    try {
      await api.delete(`/api/video-download/${id}`)
      await loadList()
    } catch (e) {
      setMsg(errMsg(e, t('Request failed')))
    }
  }

  const saveOpts = async () => {
    setOptsMsg('')
    try {
      for (const key of OPT_KEYS) {
        await api.put('/api/option/', { key, value: opts[key] })
      }
      setOptsDirty(false)
      setOptsMsg(t('Saved'))
    } catch (e) {
      setOptsMsg(errMsg(e, t('Request failed')))
    }
  }

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <div className='mx-auto w-full max-w-6xl p-4 text-sm md:p-6'>
      <h1 className='mb-4 text-xl font-semibold'>{t('Video Downloads')}</h1>

      {/* 设置面板 */}
      <div className='mb-4 rounded-lg border p-4'>
        <div className='mb-3 font-medium'>{t('Settings')}</div>
        {optsAllowed ? (
          <div className='grid grid-cols-1 gap-3 md:grid-cols-2'>
            <label className='flex items-center gap-2'>
              <input
                type='checkbox'
                checked={opts.VideoLocalizeEnabled === 'true'}
                onChange={(e) => {
                  setOpts({ ...opts, VideoLocalizeEnabled: e.target.checked ? 'true' : 'false' })
                  setOptsDirty(true)
                }}
              />
              <span>{t(OPT_LABELS.VideoLocalizeEnabled)}</span>
            </label>
            {(['VideoLocalizeConcurrency', 'VideoLocalizeTimeoutSeconds', 'VideoLocalizeMaxRetry', 'VideoLocalizeRetainDays'] as const).map(
              (key) => (
                <label key={key} className='flex flex-col gap-1'>
                  <span>{t(OPT_LABELS[key])}</span>
                  <Input
                    className='h-8 w-40'
                    value={opts[key] ?? ''}
                    onChange={(e) => {
                      setOpts({ ...opts, [key]: e.target.value })
                      setOptsDirty(true)
                    }}
                  />
                </label>
              )
            )}
            <label className='flex flex-col gap-1 md:col-span-2'>
              <span>{t(OPT_LABELS.VideoLocalizeDir)}</span>
              <Input
                className='h-8 w-full max-w-md'
                value={opts.VideoLocalizeDir ?? ''}
                onChange={(e) => {
                  setOpts({ ...opts, VideoLocalizeDir: e.target.value })
                  setOptsDirty(true)
                }}
              />
            </label>
            <label className='flex flex-col gap-1 md:col-span-2'>
              <span>{t(OPT_LABELS.VideoLocalizePublicBaseURL)}</span>
              <Input
                className='h-8 w-full max-w-lg'
                placeholder='https://your-gateway-host'
                value={opts.VideoLocalizePublicBaseURL ?? ''}
                onChange={(e) => {
                  setOpts({ ...opts, VideoLocalizePublicBaseURL: e.target.value })
                  setOptsDirty(true)
                }}
              />
            </label>
          </div>
        ) : (
          <div className='text-muted-foreground'>{t('Root permission required to change settings')}</div>
        )}
        {optsAllowed && (
          <div className='mt-3 flex items-center gap-3'>
            <Button size='sm' onClick={saveOpts} disabled={!optsDirty}>
              {t('Save')}
            </Button>
            {optsMsg && <span className='text-muted-foreground'>{optsMsg}</span>}
          </div>
        )}
      </div>

      {/* 列表筛选 */}
      <div className='mb-3 flex flex-wrap items-center gap-2'>
        <Input
          className='h-8 w-56'
          placeholder={t('Task ID')}
          value={taskId}
          onChange={(e) => {
            setTaskId(e.target.value)
            setPage(1)
          }}
        />
        <select
          className={SELECT_CLASS}
          value={status}
          onChange={(e) => {
            setStatus(e.target.value)
            setPage(1)
          }}
        >
          {STATUS_OPTIONS.map((s) => (
            <option key={s} value={s}>
              {s === '' ? t('All Status') : s}
            </option>
          ))}
        </select>
        <Button size='sm' onClick={loadList} disabled={loading}>
          {t('Search')}
        </Button>
      </div>

      {msg && (
        <div className='mb-3 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-destructive'>
          {msg}
        </div>
      )}

      <div className='overflow-x-auto rounded-lg border'>
        <table className='w-full text-sm'>
          <thead className='bg-muted text-left'>
            <tr>
              <th className='px-3 py-2 font-medium'>{t('Task ID')}</th>
              <th className='px-3 py-2 font-medium'>{t('Status')}</th>
              <th className='px-3 py-2 font-medium'>{t('Size')}</th>
              <th className='px-3 py-2 font-medium'>{t('Updated')}</th>
              <th className='px-3 py-2 font-medium'>{t('Error')}</th>
              <th className='px-3 py-2 font-medium'>{t('Actions')}</th>
            </tr>
          </thead>
          <tbody>
            {list.length === 0 && !loading ? (
              <tr>
                <td colSpan={6} className='px-3 py-6 text-center text-muted-foreground'>
                  {t('No data')}
                </td>
              </tr>
            ) : (
              list.map((v) => (
                <tr key={v.id} className='border-t'>
                  <td className='break-all px-3 py-2 font-mono text-xs'>{v.task_id}</td>
                  <td className='px-3 py-2'>
                    <span
                      className={
                        v.status === 'success'
                          ? 'text-green-600 dark:text-green-400'
                          : v.status === 'failed'
                            ? 'text-red-600 dark:text-red-400'
                            : 'text-blue-600 dark:text-blue-400'
                      }
                    >
                      {v.status}
                    </span>
                  </td>
                  <td className='px-3 py-2'>{formatSize(v.file_size)}</td>
                  <td className='px-3 py-2 text-xs text-muted-foreground'>
                    {v.updated_at ? new Date(v.updated_at).toLocaleString() : '-'}
                  </td>
                  <td className='max-w-60 break-all px-3 py-2 text-xs text-red-600 dark:text-red-400'>
                    {v.error || '-'}
                  </td>
                  <td className='px-3 py-2'>
                    <div className='flex gap-2'>
                      {v.status !== 'success' && (
                        <Button variant='outline' size='sm' onClick={() => retry(v.task_id)}>
                          {t('Retry')}
                        </Button>
                      )}
                      {v.status === 'success' && v.public_url && (
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => window.open(v.public_url, '_blank', 'noreferrer')}
                        >
                          {t('Open')}
                        </Button>
                      )}
                      <Button variant='destructive' size='sm' onClick={() => remove(v.id)}>
                        {t('Delete')}
                      </Button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className='mt-3 flex items-center justify-between'>
        <span className='text-muted-foreground'>
          {t('Total')}: {total}
        </span>
        <div className='flex items-center gap-2'>
          <Button
            variant='outline'
            size='sm'
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page <= 1 || loading}
          >
            {t('Prev')}
          </Button>
          <span className='px-2'>
            {page} / {totalPages}
          </span>
          <Button
            variant='outline'
            size='sm'
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page >= totalPages || loading}
          >
            {t('Next')}
          </Button>
        </div>
      </div>
    </div>
  )
}
