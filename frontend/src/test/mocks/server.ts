import { setupServer } from 'msw/node'
import { handlers } from '../handlers'

/** MSW Node server：在 src/test/setup.ts 中 listen / resetHandlers / close */
export const server = setupServer(...handlers)
