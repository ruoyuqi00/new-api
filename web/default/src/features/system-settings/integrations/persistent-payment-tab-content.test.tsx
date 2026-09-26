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
import assert from 'node:assert/strict'
import { test } from 'node:test'

import { renderToStaticMarkup } from 'react-dom/server'

import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { PersistentPaymentTabContent } from './persistent-payment-tab-content'

test('keeps an inactive payment form mounted with its draft values', () => {
  const markup = renderToStaticMarkup(
    <Tabs defaultValue='general'>
      <TabsList>
        <TabsTrigger value='general'>General</TabsTrigger>
        <TabsTrigger value='shkeeper'>SHKeeper</TabsTrigger>
      </TabsList>
      <PersistentPaymentTabContent value='shkeeper'>
        <input name='base_url' defaultValue='https://draft.example.com' />
      </PersistentPaymentTabContent>
    </Tabs>
  )

  assert.match(markup, /name="base_url"/)
  assert.match(markup, /value="https:\/\/draft\.example\.com"/)
})
