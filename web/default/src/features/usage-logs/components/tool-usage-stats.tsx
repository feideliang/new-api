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
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'
import { cn } from '@/lib/utils'
import { useIsAdmin } from '@/hooks/use-admin'
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { StaggerContainer, StaggerItem } from '@/components/page-transition'
import { StatCard } from '@/features/dashboard/components/ui/stat-card'
import {
  Wrench,
  Hash,
  BarChart3,
  Trophy,
  RefreshCw,
  AlertCircle,
  ListOrdered,
} from 'lucide-react'

interface ToolStat {
  tool_name: string
  call_count: number
}

type TimeRange = '7d' | '30d' | '90d' | 'all'

const TIME_RANGES: { value: TimeRange; label: string }[] = [
  { value: '7d', label: 'Last 7 days' },
  { value: '30d', label: 'Last 30 days' },
  { value: '90d', label: 'Last 90 days' },
  { value: 'all', label: 'All time' },
]

function getTimeRangeSeconds(range: TimeRange): { start: number; end: number } {
  const now = Math.floor(Date.now() / 1000)
  switch (range) {
    case '7d':
      return { start: now - 7 * 24 * 3600, end: now }
    case '30d':
      return { start: now - 30 * 24 * 3600, end: now }
    case '90d':
      return { start: now - 90 * 24 * 3600, end: now }
    case 'all':
      return { start: 0, end: now }
  }
}

export function ToolUsageStats() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const [stats, setStats] = useState<ToolStat[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(false)
  const [timeRange, setTimeRange] = useState<TimeRange>('7d')

  const fetchStats = useCallback(async (range: TimeRange) => {
    setLoading(true)
    setError(false)
    try {
      const { start, end } = getTimeRangeSeconds(range)
      const params: Record<string, unknown> = {
        start_timestamp: start,
        end_timestamp: end,
      }
      const endpoint = isAdmin ? '/api/log/tool_stat' : '/api/log/tool/self_stat'
      const res = await api.get(endpoint, { params })
      if (res.data?.success && Array.isArray(res.data.data)) {
        setStats(res.data.data as ToolStat[])
      } else {
        setStats([])
      }
    } catch {
      setStats([])
      setError(true)
    } finally {
      setLoading(false)
    }
  }, [isAdmin])

  useEffect(() => {
    void fetchStats(timeRange)
  }, [fetchStats, timeRange])

  const totalCount = useMemo(
    () => stats.reduce((sum, s) => sum + s.call_count, 0),
    [stats]
  )

  const uniqueTools = stats.length
  const topTool = stats[0]
  const topToolProportion = topTool && totalCount > 0
    ? ((topTool.call_count / totalCount) * 100).toFixed(1)
    : '0'

  return (
    <div className='flex flex-col gap-6'>
      {/* Page Header */}
      <div className='flex items-center justify-between gap-3'>
        <div className='flex min-w-0 items-center gap-2'>
          <span className='bg-muted flex size-8 shrink-0 items-center justify-center rounded-lg'>
            <Wrench className='size-4' aria-hidden='true' />
          </span>
          <div className='min-w-0'>
            <h1 className='text-xl font-semibold tracking-tight'>
              {t('Skill Usage Statistics')}
            </h1>
            <p className='text-muted-foreground line-clamp-1 text-sm'>
              {t('Tool / Skill call statistics across users and time')}
            </p>
          </div>
        </div>
        <div className='flex shrink-0 items-center gap-2'>
          <Select
            value={timeRange}
            onValueChange={(v) => setTimeRange(v as TimeRange)}
          >
            <SelectTrigger className='h-8 w-[130px] text-xs'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {TIME_RANGES.map((r) => (
                <SelectItem key={r.value} value={r.value}>
                  {t(r.label)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            variant='outline'
            size='sm'
            className='h-8 gap-1.5 px-2.5 text-xs'
            onClick={() => fetchStats(timeRange)}
            disabled={loading}
          >
            <RefreshCw
              className={cn('size-3.5', loading && 'animate-spin')}
              aria-hidden='true'
            />
            {loading ? t('Loading...') : t('Refresh')}
          </Button>
        </div>
      </div>

      {/* Summary Stat Cards */}
      <StaggerContainer className='grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4'>
        <StaggerItem>
          <Card className='h-full rounded-xl border shadow-xs'>
            <CardContent className='p-4'>
              <StatCard
                title={t('Total Calls')}
                value={loading ? '...' : totalCount.toLocaleString()}
                description={t('Total tool/skill calls in selected period')}
                icon={Hash}
                loading={loading}
                error={error}
              />
            </CardContent>
          </Card>
        </StaggerItem>

        <StaggerItem>
          <Card className='h-full rounded-xl border shadow-xs'>
            <CardContent className='p-4'>
              <StatCard
                title={t('Unique Tools')}
                value={loading ? '...' : uniqueTools.toLocaleString()}
                description={t('Number of distinct tools or skills called')}
                icon={BarChart3}
                loading={loading}
                error={error}
              />
            </CardContent>
          </Card>
        </StaggerItem>

        <StaggerItem>
          <Card className='h-full rounded-xl border shadow-xs'>
            <CardContent className='p-4'>
              <StatCard
                title={t('Top Tool')}
                value={loading || !topTool ? '—' : topTool.tool_name}
                description={
                  topTool
                    ? `${topTool.call_count.toLocaleString()} ${t('calls')}`
                    : t('No data')
                }
                icon={Trophy}
                loading={loading}
                error={error}
              />
            </CardContent>
          </Card>
        </StaggerItem>

        <StaggerItem>
          <Card className='h-full rounded-xl border shadow-xs'>
            <CardContent className='p-4'>
              <StatCard
                title={t('Top Tool Share')}
                value={loading ? '...' : `${topToolProportion}%`}
                description={t('Proportion of total calls by top tool')}
                icon={ListOrdered}
                loading={loading}
                error={error}
              />
            </CardContent>
          </Card>
        </StaggerItem>
      </StaggerContainer>

      {/* Ranking Table */}
      <Card className='rounded-xl border shadow-xs'>
        <CardHeader className='flex flex-row items-center justify-between px-5 py-4'>
          <CardTitle className='flex items-center gap-2 text-base font-semibold'>
            <ListOrdered className='text-muted-foreground size-4' aria-hidden='true' />
            {t('Call Count Ranking')}
          </CardTitle>
          {totalCount > 0 && (
            <span className='text-muted-foreground text-xs tabular-nums'>
              {t('Total')}: {totalCount.toLocaleString()} {t('calls')}
            </span>
          )}
        </CardHeader>
        <CardContent className='px-0 pb-0'>
          {loading && stats.length === 0 ? (
            <div className='flex flex-col gap-2 px-5 pb-4'>
              {Array.from({ length: 5 }).map((_, i) => (
                <div
                  key={i}
                  className='bg-muted/40 h-10 animate-pulse rounded-lg'
                />
              ))}
            </div>
          ) : error ? (
            <div className='flex flex-col items-center gap-2 px-5 pb-8 pt-4 text-center'>
              <AlertCircle className='text-muted-foreground/60 size-8' aria-hidden='true' />
              <p className='text-muted-foreground text-sm'>
                {t('Failed to load data')}
              </p>
              <Button
                variant='outline'
                size='sm'
                onClick={() => fetchStats(timeRange)}
              >
                {t('Retry')}
              </Button>
            </div>
          ) : stats.length > 0 ? (
            <div className='max-h-[500px] overflow-y-auto'>
              <Table>
                <TableHeader className='bg-muted/30 sticky top-0'>
                  <TableRow>
                    <TableHead className='w-12 text-xs font-medium'>#</TableHead>
                    <TableHead className='text-xs font-medium'>
                      {t('Tool / Skill Name')}
                    </TableHead>
                    <TableHead className='w-28 text-right text-xs font-medium'>
                      {t('Call Count')}
                    </TableHead>
                    <TableHead className='w-24 text-right text-xs font-medium'>
                      {t('Proportion')}
                    </TableHead>
                    <TableHead className='hidden w-32 text-right text-xs font-medium sm:table-cell'>
                      {t('Call Proportion')}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {stats.map((stat, index) => {
                    const proportion = totalCount > 0
                      ? (stat.call_count / totalCount) * 100
                      : 0

                    return (
                      <TableRow key={stat.tool_name} className='group'>
                        <TableCell className='text-muted-foreground font-mono text-xs tabular-nums'>
                          {index < 3 ? (
                            <span
                              className={cn(
                                'inline-flex size-5 items-center justify-center rounded text-[11px] font-bold',
                                index === 0 &&
                                  'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-400',
                                index === 1 &&
                                  'bg-slate-200 text-slate-700 dark:bg-slate-800 dark:text-slate-300',
                                index === 2 &&
                                  'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-400'
                              )}
                            >
                              {index + 1}
                            </span>
                          ) : (
                            index + 1
                          )}
                        </TableCell>
                        <TableCell className='font-mono text-sm font-medium'>
                          <span className='max-w-[200px] truncate sm:max-w-[300px] lg:max-w-[400px]'>
                            {stat.tool_name}
                          </span>
                        </TableCell>
                        <TableCell className='text-right font-mono text-sm tabular-nums'>
                          {stat.call_count.toLocaleString()}
                        </TableCell>
                        <TableCell className='text-right'>
                          <span className='inline-flex items-center gap-1.5'>
                            <span className='text-muted-foreground font-mono text-xs tabular-nums'>
                              {proportion.toFixed(1)}%
                            </span>
                            <span
                              className='bg-primary/20 dark:bg-primary/30 h-1.5 w-12 overflow-hidden rounded-full'
                              aria-hidden='true'
                            >
                              <span
                                className='bg-primary block h-full rounded-full'
                                style={{ width: `${Math.min(proportion, 100)}%` }}
                              />
                            </span>
                          </span>
                        </TableCell>
                        <TableCell className='hidden text-right font-mono text-xs tabular-nums text-muted-foreground sm:table-cell'>
                          {proportion.toFixed(1)}%
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </div>
          ) : (
            <div className='flex flex-col items-center gap-2 px-5 pb-8 pt-4 text-center'>
              <Wrench className='text-muted-foreground/40 size-8' aria-hidden='true' />
              <p className='text-muted-foreground text-sm'>
                {t('No tool usage data found. Enable LOG_REQUEST_TOOLS=true and send requests with tools.')}
              </p>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Bottom summary */}
      {stats.length > 0 && (
        <div className='text-muted-foreground/60 flex items-center justify-center gap-1 text-xs'>
          <BarChart3 className='size-3' aria-hidden='true' />
          <span>
            {stats.length} {t('tools')} · {totalCount.toLocaleString()} {t('calls')}
          </span>
        </div>
      )}
    </div>
  )
}