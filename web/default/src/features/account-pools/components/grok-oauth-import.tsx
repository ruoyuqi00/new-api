import { ArrowUpRight, KeyRound } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'

import {
  exchangeGrokOAuthAuthorization,
  generateGrokOAuthAuthorization,
} from '../api'
import type { ImportedAccount } from '../import-parser'

type GrokOAuthImportProps = {
  onAccountReady: (account: ImportedAccount) => void
}

export function GrokOAuthImport(props: GrokOAuthImportProps) {
  const { t } = useTranslation()
  const [authorization, setAuthorization] = useState<{
    authUrl: string
    sessionId: string
  }>()
  const [callbackURL, setCallbackURL] = useState('')
  const [isGenerating, setIsGenerating] = useState(false)
  const [isExchanging, setIsExchanging] = useState(false)

  const generateAuthorization = async () => {
    setIsGenerating(true)
    try {
      const response = await generateGrokOAuthAuthorization()
      if (!response.success || !response.data) {
        throw new Error(
          response.message || t('Failed to start Grok authorization')
        )
      }
      setAuthorization({
        authUrl: response.data.auth_url,
        sessionId: response.data.session_id,
      })
      setCallbackURL('')
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to start Grok authorization')
      )
    } finally {
      setIsGenerating(false)
    }
  }

  const exchangeAuthorization = async () => {
    if (!authorization || !callbackURL.trim()) return
    setIsExchanging(true)
    try {
      const response = await exchangeGrokOAuthAuthorization({
        session_id: authorization.sessionId,
        callback_url: callbackURL.trim(),
      })
      if (!response.success || !response.data) {
        throw new Error(
          response.message || t('Failed to complete Grok authorization')
        )
      }
      props.onAccountReady(response.data)
      setAuthorization(undefined)
      setCallbackURL('')
      toast.success(t('Grok account authorized'))
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to complete Grok authorization')
      )
    } finally {
      setIsExchanging(false)
    }
  }

  return (
    <section className='border-primary/25 min-w-0 space-y-4 border-l-2 p-4'>
      <Alert>
        <KeyRound />
        <AlertTitle>{t('Grok OAuth')}</AlertTitle>
        <AlertDescription>
          {t('Use your account password only on the official xAI page.')}
        </AlertDescription>
      </Alert>

      {!authorization ? (
        <Button
          type='button'
          variant='outline'
          disabled={isGenerating}
          onClick={generateAuthorization}
        >
          {isGenerating ? (
            <Spinner data-icon='inline-start' />
          ) : (
            <KeyRound data-icon='inline-start' />
          )}
          {t('Generate authorization link')}
        </Button>
      ) : (
        <FieldGroup>
          <div className='flex flex-wrap gap-2'>
            <Button
              render={
                <a
                  href={authorization.authUrl}
                  target='_blank'
                  rel='noreferrer'
                />
              }
            >
              <ArrowUpRight data-icon='inline-start' />
              {t('Open xAI authorization')}
            </Button>
            <Button
              type='button'
              variant='ghost'
              onClick={generateAuthorization}
              disabled={isGenerating}
            >
              {t('Generate a new link')}
            </Button>
          </div>
          <Field>
            <FieldLabel htmlFor='grok-oauth-callback'>
              {t('Authorization callback URL')}
            </FieldLabel>
            <Textarea
              id='grok-oauth-callback'
              rows={3}
              value={callbackURL}
              onChange={(event) => setCallbackURL(event.target.value)}
              placeholder={t('Paste the full callback URL')}
            />
            <FieldDescription>
              {t(
                'After authorization, paste the full callback URL from the browser address bar.'
              )}
            </FieldDescription>
          </Field>
          <Button
            type='button'
            disabled={!callbackURL.trim() || isExchanging}
            onClick={exchangeAuthorization}
          >
            {isExchanging ? (
              <Spinner data-icon='inline-start' />
            ) : (
              <KeyRound data-icon='inline-start' />
            )}
            {t('Exchange and add account')}
          </Button>
        </FieldGroup>
      )}
    </section>
  )
}
