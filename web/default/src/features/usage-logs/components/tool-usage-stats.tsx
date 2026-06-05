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
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'

interface ToolStat {
  tool_name: string
  call_count: number
}

export function ToolUsageStats() {
  const { t } = useTranslation()
  const [stats, setStats] = useState<ToolStat[]>([])
  const [loading, setLoading] = useState(false)

  const fetchStats = useCallback(async () => {
    setLoading(true)
    try {
      // Last 7 days by default
      const now = Math.floor(Date.now() / 1000)
      const sevenDaysAgo = now - 7 * 24 * 3600
      const params = {
        start_timestamp: sevenDaysAgo,
        end_timestamp: now,
      }
      const res = await api.get('/api/log/tool_stat', { params })
      if (res.data?.success && Array.isArray(res.data.data)) {
        setStats(res.data.data as ToolStat[])
      } else {
        setStats([])
      }
    } catch {
      setStats([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchStats()
  }, [fetchStats])

  const totalCount = stats.reduce((sum, s) => sum + s.call_count, 0)

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <CardTitle className='text-lg'>{t('Skill Usage Statistics')}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className='flex items-center gap-4 mb-4'>
            <Button onClick={fetchStats} disabled={loading} size='sm'>
              {loading ? t('Loading...') : t('Refresh')}
            </Button>
            {totalCount > 0 && (
              <span className='text-sm text-muted-foreground'>
                {t('Total')}: {totalCount} {t('calls')} ({t('Last 7 days')})
              </span>
            )}
          </div>

          {stats.length > 0 ? (
            <ScrollArea className='max-h-[600px]'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className='w-12'>#</TableHead>
                    <TableHead>{t('Tool / Skill Name')}</TableHead>
                    <TableHead className='text-right'>{t('Call Count')}</TableHead>
                    <TableHead className='text-right'>{t('Proportion')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {stats.map((stat, index) => (
                    <TableRow key={stat.tool_name}>
                      <TableCell className='text-muted-foreground'>{index + 1}</TableCell>
                      <TableCell className='font-mono text-sm'>{stat.tool_name}</TableCell>
                      <TableCell className='text-right font-mono'>{stat.call_count}</TableCell>
                      <TableCell className='text-right text-muted-foreground'>
                        {totalCount > 0
                          ? `${((stat.call_count / totalCount) * 100).toFixed(1)}%`
                          : '0%'}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </ScrollArea>
          ) : (
            <p className='text-sm text-muted-foreground py-8 text-center'>
              {loading
                ? t('Loading...')
                : t('No tool usage data found.')}
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}