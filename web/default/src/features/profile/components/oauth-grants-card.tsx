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
import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { RefreshCw, ShieldCheck, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import dayjs from '@/lib/dayjs'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
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
import {
  getOAuthServerGrants,
  revokeOAuthServerGrant,
  type OAuthServerUserGrant,
} from '../api'

const oauthGrantsQueryKey = ['oauth-server-grants'] as const

export function OAuthGrantsCard() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [revokeTarget, setRevokeTarget] = useState<OAuthServerUserGrant | null>(
    null
  )

  const grantsQuery = useQuery({
    queryKey: oauthGrantsQueryKey,
    queryFn: getOAuthServerGrants,
  })

  const grants = useMemo(
    () => grantsQuery.data?.data ?? [],
    [grantsQuery.data?.data]
  )

  const revokeMutation = useMutation({
    mutationFn: revokeOAuthServerGrant,
    onSuccess: async (res) => {
      if (!res.success) {
        toast.error(res.message || t('Failed to revoke OAuth grant'))
        return
      }
      toast.success(t('OAuth grant revoked'))
      setRevokeTarget(null)
      await queryClient.invalidateQueries({ queryKey: oauthGrantsQueryKey })
    },
    onError: () => {
      toast.error(t('Failed to revoke OAuth grant'))
    },
  })

  const isLoading = grantsQuery.isLoading
  const isRefreshing = grantsQuery.isFetching && !grantsQuery.isLoading

  return (
    <>
      <Card className='gap-0 overflow-hidden py-0'>
        <CardHeader className='flex flex-col gap-3 p-3 sm:flex-row sm:items-start sm:justify-between sm:p-5'>
          <div className='min-w-0 space-y-1'>
            <CardTitle className='text-lg tracking-tight sm:text-xl'>
              {t('OAuth App Grants')}
            </CardTitle>
            <CardDescription className='text-xs sm:text-sm'>
              {t('Review apps authorized to access your account.')}
            </CardDescription>
          </div>
          <Button
            type='button'
            variant='outline'
            size='icon'
            className='shrink-0'
            disabled={isRefreshing}
            aria-label={t('Refresh')}
            onClick={() => grantsQuery.refetch()}
          >
            <RefreshCw
              className={isRefreshing ? 'h-4 w-4 animate-spin' : 'h-4 w-4'}
            />
          </Button>
        </CardHeader>

        <CardContent className='p-3 sm:p-5'>
          {isLoading ? (
            <div className='space-y-3'>
              <Skeleton className='h-20 w-full' />
              <Skeleton className='h-20 w-full' />
            </div>
          ) : grantsQuery.data && !grantsQuery.data.success ? (
            <div className='border-border bg-muted/30 rounded-md border p-4 text-sm'>
              <p className='font-medium'>{t('Failed to load OAuth grants')}</p>
              <p className='text-muted-foreground mt-1'>
                {grantsQuery.data.message || t('Request failed')}
              </p>
            </div>
          ) : grants.length === 0 ? (
            <div className='border-border bg-muted/30 flex flex-col items-center gap-3 rounded-md border p-6 text-center'>
              <div className='bg-background rounded-md p-2'>
                <ShieldCheck className='h-5 w-5' />
              </div>
              <div className='space-y-1'>
                <p className='font-medium'>{t('No OAuth app grants')}</p>
                <p className='text-muted-foreground text-sm'>
                  {t('Approved OAuth apps will appear here.')}
                </p>
              </div>
            </div>
          ) : (
            <div className='space-y-3'>
              {grants.map((grant) => (
                <div
                  key={grant.id}
                  className='border-border flex flex-col gap-4 rounded-md border p-4 sm:flex-row sm:items-start sm:justify-between'
                >
                  <div className='min-w-0 space-y-3'>
                    <div className='space-y-1'>
                      <div className='flex flex-wrap items-center gap-2'>
                        <p className='font-medium'>{grant.client_name}</p>
                        <StatusBadge
                          label={t('Enabled')}
                          variant='success'
                          showDot
                          copyable={false}
                        />
                      </div>
                      <p className='text-muted-foreground text-xs break-all'>
                        {t('Client ID')}: {grant.client_id}
                      </p>
                    </div>

                    <div className='flex flex-wrap gap-1.5'>
                      {grant.scopes.map((scope) => (
                        <Badge key={scope} variant='secondary'>
                          {scope}
                        </Badge>
                      ))}
                    </div>

                    <dl className='text-muted-foreground grid gap-1 text-xs sm:grid-cols-2'>
                      <div>
                        <dt className='inline'>{t('Last used')}: </dt>
                        <dd className='inline'>
                          {grant.last_used_at
                            ? dayjs(grant.last_used_at).fromNow()
                            : t('Never used')}
                        </dd>
                      </div>
                      <div>
                        <dt className='inline'>{t('Granted')}: </dt>
                        <dd className='inline'>
                          {dayjs(grant.created_at).fromNow()}
                        </dd>
                      </div>
                    </dl>
                  </div>

                  <Button
                    type='button'
                    variant='destructive'
                    size='sm'
                    className='shrink-0'
                    disabled={revokeMutation.isPending}
                    onClick={() => setRevokeTarget(grant)}
                  >
                    <Trash2 className='h-4 w-4' />
                    {t('Revoke access')}
                  </Button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <AlertDialog
        open={!!revokeTarget}
        onOpenChange={(open) => {
          if (!open) setRevokeTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Revoke OAuth grant')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('Revoke access for {{client}}?', {
                client: revokeTarget?.client_name ?? '',
              })}
              <br />
              {t('This will revoke active OAuth tokens for this app.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              disabled={revokeMutation.isPending || !revokeTarget}
              onClick={(event) => {
                event.preventDefault()
                if (!revokeTarget) return
                revokeMutation.mutate(revokeTarget.client_id)
              }}
            >
              {t('Revoke access')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
