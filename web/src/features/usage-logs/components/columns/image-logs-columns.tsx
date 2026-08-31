/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { ColumnDef } from '@tanstack/react-table'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'
import { cn } from '@/lib/utils'

import type { UsageLog } from '../../data/schema'
import { formatModelName, parseLogOther } from '../../lib/format'
import { DetailsDialog } from '../dialogs/details-dialog'
import { LogCostDisplay } from '../log-cost-display'
import { ModelBadge } from '../model-badge'
import { useUsageLogsContext } from '../usage-logs-provider'
import { createChannelColumn, createTimestampColumn } from './column-helpers'

/**
 * Image-generation log columns.
 *
 * 生图日志是经 `image_only` 过滤后从 Log 表浮出的 Consume（type=2）行，
 * 形状与 UsageLog 一致。列复用 common logs 的原语（ModelBadge /
 * LogCostDisplay / DetailsDialog），但略去 type/stream/timing 等对同步生图
 * 调用无意义的列。
 */
export function useImageLogsColumns(isAdmin: boolean): ColumnDef<UsageLog>[] {
  const { t } = useTranslation()

  const columns: ColumnDef<UsageLog>[] = [
    createTimestampColumn<UsageLog>({
      accessorKey: 'created_at',
      title: t('Time'),
      unit: 'milliseconds',
    }),
  ]

  if (isAdmin) {
    columns.push(
      createChannelColumn<UsageLog>({
        accessorKey: 'channel',
        headerLabel: t('Channel'),
      }),
      {
        id: 'user',
        header: t('User'),
        accessorFn: (row) => row.username || row.user_id,
        cell: function UserCell({ row }) {
          const { sensitiveVisible, setSelectedUserId, setUserInfoDialogOpen } =
            useUsageLogsContext()
          const log = row.original
          const displayName = log.username || String(log.user_id || '?')

          return (
            <button
              type='button'
              className='flex items-center gap-1.5 text-left'
              onClick={(e) => {
                e.stopPropagation()
                setSelectedUserId(log.user_id)
                setUserInfoDialogOpen(true)
              }}
            >
              <Avatar className='ring-border/60 size-6 ring-1 max-sm:hidden'>
                <AvatarFallback
                  className={cn(
                    'text-[11px] font-semibold',
                    !sensitiveVisible && 'bg-muted text-muted-foreground'
                  )}
                  style={
                    sensitiveVisible ? getUserAvatarStyle(displayName) : undefined
                  }
                >
                  {sensitiveVisible ? getUserAvatarFallback(displayName) : '•'}
                </AvatarFallback>
              </Avatar>
              <span className='text-muted-foreground truncate text-sm hover:underline'>
                {sensitiveVisible ? displayName : '••••'}
              </span>
            </button>
          )
        },
      }
    )
  }

  columns.push(
    {
      accessorKey: 'model_name',
      header: t('Model'),
      cell: function ModelCell({ row }) {
        const log = row.original
        const modelInfo = formatModelName(log)

        return (
          <div className='flex w-fit flex-col gap-0.5'>
            <ModelBadge
              modelName={modelInfo.name}
              actualModel={modelInfo.actualModel}
            />
          </div>
        )
      },
      meta: { mobileTitle: true },
    },
    {
      accessorKey: 'content',
      header: t('Details'),
      cell: function DetailsCell({ row }) {
        const [dialogOpen, setDialogOpen] = useState(false)
        const log = row.original
        const text = log.content?.trim()

        return (
          <>
            <button
              type='button'
              className='group flex max-w-[200px] items-center gap-1 text-left text-xs'
              onClick={() => setDialogOpen(true)}
              title={t('Click to view full details')}
            >
              {text ? (
                <span className='truncate leading-snug text-foreground group-hover:underline'>
                  {log.content}
                </span>
              ) : (
                <span className='text-muted-foreground/40'>—</span>
              )}
            </button>
            <DetailsDialog
              log={log}
              isAdmin={isAdmin}
              open={dialogOpen}
              onOpenChange={setDialogOpen}
            />
          </>
        )
      },
      size: 180,
      maxSize: 200,
    },
    {
      accessorKey: 'quota',
      header: t('Cost'),
      cell: ({ row }) => {
        const log = row.original
        const quota = row.getValue('quota') as number
        const other = parseLogOther(log.other)
        return <LogCostDisplay quota={quota} other={other} />
      },
    }
  )

  return columns
}
