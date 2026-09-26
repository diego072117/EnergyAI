# Guion de la demo (≈ 7 minutos)

Preparación: `docker compose up` con `LLM_PROVIDER=ollama` y el modelo `qwen2.5:7b` descargado. Para empezar sin análisis previos, reinicia la base de datos con `docker compose down -v && docker compose up -d`.

| Tiempo | Pantalla | Qué mostrar / decir |
|---|---|---|
| 0:00 | — | "Una plataforma que convierte lecturas en decisiones: qué pasa, qué se sale de lo normal, si es real, qué atender primero, por qué y qué hacer." |
| 0:30 | Login | Usuario demo. El panel izquierdo resume lo que aporta la IA. |
| 1:00 | Dashboard vacío | 12 medidores y 155 MWh en 14 días, todavía sin análisis. |
| 1:30 | **Run AI Analysis** | Los 7 pasos en vivo: baseline, detección, correlación, eventos y la explicación redactada por el LLM local. Resultado: **"4 anomalías detectadas · 2 requieren atención prioritaria"**. |
| 2:30 | Dashboard | KPIs, "Requiere atención" con **M-109 en primer lugar**, y "Hallazgos por tipo": la IA también descarta. |
| 3:00 | Medidores → M-109 | +110% frente al baseline. Gráfico con la banda normal, la ventana de la anomalía y la marca "Sin evento reportado". Pestañas de corriente y FP: la corriente se duplica y el FP cae. |
| 4:00 | Investigación M-109 | Qué encontró la IA, variables antes y después, evidencia 6/6, el evento `UNKNOWN` que no explica el cambio, desglose de confianza y prioridad. Clic en **Marcar en investigación** con una nota. |
| 5:30 | Anomalías IA | Contraste: M-112 es un problema de datos (voltaje fuera de banda con consumo estable), M-104 lo explica la nueva línea y M-106 es un falso positivo por la parada de 12 h. |
| 6:30 | Cierre | Motor determinista y probado, LLM solo para redactar y con guardrails, funciona sin API key, arquitectura, tests y CI. |
