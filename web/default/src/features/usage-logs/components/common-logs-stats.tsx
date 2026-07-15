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
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import { formatLogQuota } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useIsAdmin } from '@/hooks/use-admin'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { getLogStats, getUserLogStats } from '../api'
import { DEFAULT_LOG_STATS } from '../constants'
import { buildApiParams } from '../lib/utils'
import { useLogsViewScope, useUsageLogsContext } from './usage-logs-provider'
import type { QuotaUsageDetail } from '../types'

const route = getRouteApi('/_authenticated/usage-logs/$section')

function StatBadge(props: {
  label: string
  value: string | number
  accent: string
}) {
  return (
    <span className='border-border/60 bg-muted/25 inline-flex h-7 items-center gap-2 rounded-md border px-2.5 text-xs shadow-xs'>
      <span className={cn('h-3.5 w-0.5 rounded-full', props.accent)} />
      <span className='text-muted-foreground'>{props.label}</span>
      <span className='text-foreground/85 font-mono font-semibold tabular-nums'>
        {props.value}
      </span>
    </span>
  )
}

export function CommonLogsStats() {
  const { t } = useTranslation()
  const { isAdminView: isAdmin } = useLogsViewScope()
  const searchParams = route.useSearch()
  const { sensitiveVisible } = useUsageLogsContext()
  const [showDetails, setShowDetails] = useState(false)

  const { data: stats, isLoading } = useQuery({
    queryKey: ['usage-logs-stats', isAdmin, searchParams],
    queryFn: async () => {
      const params = buildApiParams({
        page: 1,
        pageSize: 1,
        searchParams,
        columnFilters: [],
        isAdmin,
      })

      const result = isAdmin
        ? await getLogStats(params)
        : await getUserLogStats(params)

      return result.success
        ? result.data || DEFAULT_LOG_STATS
        : DEFAULT_LOG_STATS
    },
    placeholderData: (previousData) => previousData,
  })

  const details: QuotaUsageDetail[] = stats?.details || []

  if (isLoading) {
    return (
      <div className='flex items-center gap-2'>
        <Skeleton className='h-7 w-[150px] rounded-md' />
        <Skeleton className='h-7 w-[100px] rounded-md' />
        <Skeleton className='h-7 w-[120px] rounded-md' />
      </div>
    )
  }

  return (
    <>
      <div className='flex flex-wrap items-center gap-2'>
        <StatBadge
          label={t('Usage')}
          value={sensitiveVisible ? formatLogQuota(stats?.quota || 0) : '••••'}
          accent='bg-sky-500/70'
        />
        <StatBadge
          label={t('RPM')}
          value={stats?.rpm || 0}
          accent='bg-rose-500/65'
        />
        <StatBadge
          label={t('TPM')}
          value={stats?.tpm || 0}
          accent='bg-slate-400/70'
        />
        {details.length > 0 && (
          <button
            type='button'
            onClick={() => setShowDetails(!showDetails)}
            className='border-border/60 bg-muted/25 hover:bg-muted/40 inline-flex h-7 items-center gap-1 rounded-md border px-2.5 text-xs shadow-xs transition-colors'
          >
            {showDetails ? '▴' : '▾'} <span className='text-muted-foreground'>{t('Details')}</span>
          </button>
        )}
      </div>

      {/* Usage Detail by User + Key */}
      {showDetails && details.length > 0 && (
        <Card className='mt-2 rounded-xl border shadow-xs'>
          <CardHeader className='px-4 py-3'>
            <CardTitle className='text-xs font-semibold'>
              {t('Usage by User & Key')}
            </CardTitle>
          </CardHeader>
          <CardContent className='px-0 pb-0'>
            <div className='max-h-60 overflow-auto'>
              <Table>
                <TableHeader className='bg-muted/30 sticky top-0'>
                  <TableRow>
                    <TableHead className='text-xs'>{t('User')}</TableHead>
                    <TableHead className='text-xs'>{t('Key')}</TableHead>
                    <TableHead className='text-right text-xs'>{t('Requests')}</TableHead>
                    <TableHead className='text-right text-xs'>{t('Prompt Tokens')}</TableHead>
                    <TableHead className='text-right text-xs'>{t('Completion Tokens')}</TableHead>
                    <TableHead className='text-right text-xs'>{t('Total Tokens')}</TableHead>
                    <TableHead className='text-right text-xs'>{t('Usage')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {details.map((d, idx) => (
                    <TableRow key={`${d.user_id}-${d.token_id}-${idx}`}>
                      <TableCell className='text-xs'>
                        <div className='font-medium'>{d.username || '—'}</div>
                        <div className='text-muted-foreground font-mono'>ID: {d.user_id || '—'}</div>
                      </TableCell>
                      <TableCell className='text-xs'>
                        <div className='font-medium'>{d.token_name || '—'}</div>
                        <div className='text-muted-foreground font-mono'>ID: {d.token_id || '—'}</div>
                      </TableCell>
                      <TableCell className='text-right font-mono text-xs tabular-nums'>
                        {d.count.toLocaleString()}
                      </TableCell>
                      <TableCell className='text-right font-mono text-xs tabular-nums'>
                        {d.prompt_tokens.toLocaleString()}
                      </TableCell>
                      <TableCell className='text-right font-mono text-xs tabular-nums'>
                        {d.completion_tokens.toLocaleString()}
                      </TableCell>
                      <TableCell className='text-right font-mono text-xs tabular-nums'>
                        {d.total_tokens.toLocaleString()}
                      </TableCell>
                      <TableCell className='text-right font-mono text-xs tabular-nums'>
                        {sensitiveVisible ? formatLogQuota(d.quota || 0) : '••••'}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </CardContent>
        </Card>
      )}
    </>
  )
}