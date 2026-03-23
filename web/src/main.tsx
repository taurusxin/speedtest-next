import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { MantineProvider, createTheme } from '@mantine/core'
import '@mantine/core/styles.css'
import './index.css'
import App from './App.tsx'

const theme = createTheme({
  primaryColor: 'cyan',
  fontFamily: "'Manrope', 'Segoe UI', sans-serif",
  headings: {
    fontFamily: "'Space Grotesk', 'Segoe UI', sans-serif",
  },
  radius: {
    xl: '24px',
  },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <MantineProvider theme={theme} defaultColorScheme="auto">
      <App />
    </MantineProvider>
  </StrictMode>,
)
