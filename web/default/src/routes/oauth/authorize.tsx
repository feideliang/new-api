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
import { useEffect, useState } from 'react'
import { createFileRoute, useSearch, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api } from '@/lib/api'

type AuthorizeMeta = {
  success: boolean
  client_id: string
  client_name: string
  redirect_uri: string
  state: string
  scopes: string[]
  scope: string
  code_challenge: string
  code_challenge_method: string
  nonce: string
}

function buildRedirectURI(redirectURI: string, params: Record<string, string>) {
  const sep = redirectURI.includes('?') ? '&' : '?'
  const usp = new URLSearchParams(params).toString()
  return `${redirectURI}${sep}${usp}`
}

function OAuthAuthorize() {
  const { t } = useTranslation()
  const search = useSearch({ from: '/oauth/authorize' }) as Record<
    string,
    string
  >
  const navigate = useNavigate()

  const [meta, setMeta] = useState<AuthorizeMeta | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    if (!search?.client_id) {
      setError(t('Invalid OAuth request: missing client_id'))
      return
    }
    ;(async () => {
      try {
        const resp = await api.get<AuthorizeMeta>('/api/oauth/authorize/meta', {
          params: { ...search },
          skipErrorHandler: true,
          skipBusinessError: true,
        })
        if (resp.data?.success) {
          setMeta(resp.data)
        } else {
          setError(`${t('Failed to load authorization info')} (${resp.status})`)
        }
      } catch (e: unknown) {
        const errObj = e as {
          response?: { status?: number; data?: unknown }
          message?: string
        }
        const status = errObj?.response?.status
        if (status === 401) {
          const redirect = encodeURIComponent(
            '/oauth/authorize?' + new URLSearchParams(search).toString()
          )
          window.location.replace(`/sign-in?redirect=${redirect}`)
          return
        }
        const detail =
          errObj?.message ||
          JSON.stringify(errObj?.response?.data ?? {}) ||
          'unknown'
        setError(`${t('Failed to load authorization info')}: ${detail}`)
      }
    })()
  }, [search, t, navigate])

  const handleApprove = async () => {
    if (!meta) return
    setSubmitting(true)
    try {
      // Use a native form submit so the browser handles the 302 redirect
      // to the callback URL automatically (in the same tab). This avoids
      // any axios / header size quirks that could truncate the Location URL.
      const form = document.createElement('form')
      form.method = 'POST'
      form.action = '/oauth/authorize'
      form.rel = 'noopener'
      for (const [key, value] of Object.entries(search ?? {})) {
        if (value == null) continue
        const input = document.createElement('input')
        input.type = 'hidden'
        input.name = key
        input.value = String(value)
        form.appendChild(input)
      }
      const decision = document.createElement('input')
      decision.type = 'hidden'
      decision.name = 'decision'
      decision.value = 'approve'
      form.appendChild(decision)
      document.body.appendChild(form)
      form.target = '_blank'
      form.submit()
      // The new tab carries the redirect. Close the consent screen shortly
      // after, but only if we managed to open a popup (it can be blocked).
      setTimeout(() => window.close(), 100)
    } catch (e) {
      toast.error(t('Authorization failed'))
      void e
      setSubmitting(false)
    }
  }

  const handleDeny = () => {
    if (!meta) return
    const target = buildRedirectURI(meta.redirect_uri, {
      error: 'access_denied',
      error_description: 'The user denied the authorization request.',
      ...(meta.state ? { state: meta.state } : {}),
    })
    const opened = window.open(target, '_blank', 'noopener,noreferrer')
    if (opened) {
      window.close()
    } else {
      window.location.replace(target)
    }
  }

  if (error) {
    return (
      <div className='bg-surface flex min-h-screen items-center justify-center px-4'>
        <div className='border-border bg-surface-1 max-w-md rounded-lg border p-6 text-center'>
          <h1 className='text-text mb-2 text-lg font-semibold'>
            {t('Authorization error')}
          </h1>
          <p className='text-text-muted text-sm'>{error}</p>
        </div>
      </div>
    )
  }

  if (!meta) {
    return (
      <div className='bg-surface flex min-h-screen items-center justify-center'>
        <div className='text-text-muted text-sm'>{t('Loading…')}</div>
      </div>
    )
  }

  return (
    <div className='bg-surface flex min-h-screen items-center justify-center px-4'>
      <div className='border-border bg-surface-1 w-full max-w-md rounded-xl border p-8 shadow-sm'>
        <h1 className='text-text mb-1 text-center text-xl font-semibold'>
          {meta.client_name || meta.client_id}
        </h1>
        <p className='text-text-muted mb-6 text-center text-sm'>
          {t('wants to access your account')}
        </p>

        <div className='border-border bg-surface-2 mb-6 rounded-lg border p-4'>
          <div className='text-text-muted mb-2 text-xs font-medium tracking-wide uppercase'>
            {t('Permissions requested')}
          </div>
          <ul className='text-text space-y-2 text-sm'>
            {meta.scopes.map((s) => (
              <li key={s} className='flex items-center gap-2'>
                <span className='bg-accent inline-block h-1.5 w-1.5 rounded-full' />
                <span>{describeScope(s, t)}</span>
              </li>
            ))}
          </ul>
        </div>

        <div className='text-text-muted mb-4 text-xs'>
          {t('Redirect to')}:{' '}
          <span className='break-all'>{meta.redirect_uri}</span>
        </div>

        <div className='flex gap-3'>
          <button
            type='button'
            onClick={handleDeny}
            disabled={submitting}
            className='border-border bg-surface text-text hover:bg-surface-2 flex-1 rounded-md border px-4 py-2 text-sm font-medium transition disabled:opacity-50'
          >
            {t('Deny')}
          </button>
          <button
            type='button'
            onClick={handleApprove}
            disabled={submitting}
            className='bg-accent text-on-accent flex-1 rounded-md px-4 py-2 text-sm font-medium transition hover:opacity-90 disabled:opacity-50'
          >
            {submitting ? t('Authorizing…') : t('Authorize')}
          </button>
        </div>
      </div>
    </div>
  )
}

function describeScope(scope: string, t: (k: string) => string): string {
  const known: Record<string, string> = {
    openid: t('Sign you in'),
    profile: t('View your basic profile'),
    email: t('View your email address'),
    offline_access: t(
      'Access your account when you are not using the application'
    ),
    'api.connectors.read': t('Read connector information'),
    'api.connectors.invoke': t('Invoke connectors on your behalf'),
  }
  return known[scope] ?? scope
}

export const Route = createFileRoute('/oauth/authorize')({
  component: OAuthAuthorize,
})
