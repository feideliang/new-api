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
import type { ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { RefreshCw, ServerCog } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { StatusBadge } from '@/components/status-badge'
import { getOAuthServerStatus } from '../api'
import { SettingsSection } from '../components/settings-section'

export function OAuthServerSection() {
  const { t } = useTranslation()
  const statusQuery = useQuery({
    queryKey: ['oauth-server-admin-status'],
    queryFn: getOAuthServerStatus,
  })

  const status = statusQuery.data?.data
  const client = status?.codex_client
  const isRefreshing = statusQuery.isFetching && !statusQuery.isLoading

  return (
    <SettingsSection title={t('OAuth Server')}>
      <Card>
        <CardHeader className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
          <div className='min-w-0 space-y-1'>
            <CardTitle className='flex items-center gap-2'>
              <ServerCog className='h-5 w-5' />
              {t('OAuth Server Status')}
            </CardTitle>
            <CardDescription>
              {t('Review the built-in OAuth server and Codex client status.')}
            </CardDescription>
          </div>
          <Button
            type='button'
            variant='outline'
            size='icon'
            disabled={isRefreshing}
            aria-label={t('Refresh')}
            onClick={() => statusQuery.refetch()}
          >
            <RefreshCw
              className={isRefreshing ? 'h-4 w-4 animate-spin' : 'h-4 w-4'}
            />
          </Button>
        </CardHeader>
        <CardContent className='space-y-5'>
          {statusQuery.isLoading ? (
            <div className='space-y-3'>
              <Skeleton className='h-16 w-full' />
              <Skeleton className='h-28 w-full' />
            </div>
          ) : !statusQuery.data?.success || !status ? (
            <div className='border-border bg-muted/30 rounded-md border p-4 text-sm'>
              <p className='font-medium'>
                {t('Failed to load OAuth server status')}
              </p>
              <p className='text-muted-foreground mt-1'>
                {statusQuery.data?.message || t('Request failed')}
              </p>
            </div>
          ) : (
            <>
              <div className='grid gap-3 sm:grid-cols-3'>
                <StatusItem
                  label={t('Server')}
                  value={
                    <StatusBadge
                      label={status.enabled ? t('Enabled') : t('Disabled')}
                      variant={status.enabled ? 'success' : 'neutral'}
                      showDot
                      copyable={false}
                    />
                  }
                />
                <StatusItem
                  label={t('Signing key')}
                  value={
                    <StatusBadge
                      label={
                        status.signing_key_configured
                          ? t('Configured')
                          : t('Not configured')
                      }
                      variant={
                        status.signing_key_configured ? 'success' : 'warning'
                      }
                      showDot
                      copyable={false}
                    />
                  }
                />
                <StatusItem
                  label={t('Codex client')}
                  value={
                    <StatusBadge
                      label={client?.enabled ? t('Enabled') : t('Disabled')}
                      variant={client?.enabled ? 'success' : 'neutral'}
                      showDot
                      copyable={false}
                    />
                  }
                />
              </div>

              {status.error && (
                <div className='border-border bg-muted/30 rounded-md border p-4 text-sm'>
                  <p className='font-medium'>{t('Configuration notice')}</p>
                  <p className='text-muted-foreground mt-1'>{status.error}</p>
                </div>
              )}

              <div className='border-border rounded-md border'>
                <div className='border-border border-b p-4'>
                  <p className='font-medium'>{client?.client_name}</p>
                  <p className='text-muted-foreground mt-1 text-sm break-all'>
                    {t('Client ID')}: {client?.client_id}
                  </p>
                </div>
                <div className='grid gap-4 p-4 lg:grid-cols-2'>
                  <DetailBlock
                    label={t('Issuer')}
                    values={[status.issuer || t('Not configured')]}
                  />
                  <DetailBlock
                    label={t('Signing key ID')}
                    values={[status.signing_key_id || t('Not available')]}
                  />
                  <DetailBlock
                    label={t('Redirect URIs')}
                    values={client?.redirect_uris ?? []}
                  />
                  <DetailBlock
                    label={t('Allowed scopes')}
                    values={client?.allowed_scopes ?? []}
                    badges
                  />
                </div>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </SettingsSection>
  )
}

function StatusItem({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className='border-border bg-muted/20 rounded-md border p-4'>
      <p className='text-muted-foreground mb-2 text-xs font-medium'>{label}</p>
      {value}
    </div>
  )
}

function DetailBlock({
  label,
  values,
  badges = false,
}: {
  label: string
  values: string[]
  badges?: boolean
}) {
  const { t } = useTranslation()
  const displayValues = values.length > 0 ? values : [t('None')]

  return (
    <div className='min-w-0 space-y-2'>
      <p className='text-muted-foreground text-xs font-medium'>{label}</p>
      {badges ? (
        <div className='flex flex-wrap gap-1.5'>
          {displayValues.map((value) => (
            <Badge key={value} variant='secondary'>
              {value}
            </Badge>
          ))}
        </div>
      ) : (
        <div className='space-y-1'>
          {displayValues.map((value) => (
            <p key={value} className='text-sm break-all'>
              {value}
            </p>
          ))}
        </div>
      )}
    </div>
  )
}
