# Motor de anomalías

Código: `backend/internal/analytics`. Es un paquete puro: `Run(cfg, inputs) → findings`.

```
Lecturas → Baseline → Detección → Correlación → Eventos → Clasificación → Explicación → Recomendación
```

## 1. Validación

Ordena por tiempo, descarta duplicados y valores físicamente imposibles (NaN, kWh negativos, FP fuera de [0,1]) y cuenta las horas faltantes. Si hay 3 o más incidencias, cuentan como una señal más de calidad de datos.

## 2. Baseline

- **Ventana de referencia:** los primeros `BaselineDays = 7` días de cada medidor.
- Para cada hora del día (0–23) y cada variable (kWh, V, I, FP) se calculan la **mediana y la MAD**. Son robustas a outliers y respetan el perfil diario.
- **Baseline diario** = suma de las 24 medianas horarias.
- **Relación de potencia** propia del medidor: mediana de `V·I·FP/1000 ÷ kWh`. En un medidor sano es estable (≈0,90–0,98 en este dataset).
- **Consumo actual** = últimas 24 h. **Variación** = actual / baseline − 1.

## 3. Detectores

| Detector | Regla | Parámetros |
|---|---|---|
| **Cambio de nivel** (subidas persistentes, caídas, paradas) | Desviación relativa horaria frente a la mediana de esa hora. Media móvil de 6 h. Si queda fuera de ±25% durante ≥ 6 h seguidas, se abre un segmento. Como la media móvil tiene retraso, los bordes se ajustan a la primera y la última lectura individual fuera del umbral | `ShiftThreshold=0.25`, `ShiftWindowHours=6`, `ShiftMinHours=6` |
| **Picos aislados** | z robusto > 4 **y** \|desviación\| > 30%, fuera de los segmentos | `SpikeZ=4`, `SpikeMinDev=0.30` |
| **Calidad de datos** | Voltaje fuera de 220 V ±5%. `V·I·FP/kWh` desviada más del 35% de la relación propia del medidor. Saltos de voltaje mayores que la banda entre lecturas consecutivas. Un chequeo es *señal* si ocurre ≥ 3 veces; hacen falta ≥ 2 señales | `VoltageTolerance=0.05`, `PowerRatioTolerance=0.35`, `DQMinOccurrences=3`, `DQMinSignals=2` |
| **Relación eléctrica** | En la ventana del cambio: ¿la corriente se mueve en proporción al consumo (diferencia de ratios ≤ 20%)? ¿El FP cae ≥ 0,10? ¿El voltaje cambia ≥ 1%? | `CoMoveTolerance=0.20`, `PFDropThreshold=0.10`, `VoltageChangePct=1` |

**Por qué el z-score solo no basta:** con 7 muestras por hora la MAD es muy pequeña, y un ±15% de ruido normal alcanza z = 24. Exigir además un 30% de desviación relativa elimina esos falsos positivos. Lo verifica `TestSmallNoiseDoesNotTriggerSpikes`.

**Justificación de los umbrales:** ±5% es la tolerancia típica de tensión de suministro. Un 25% sostenido durante 6 h es muy superior al ruido observado (máximo 22% en una sola hora en los medidores normales) y más corto que cualquier evento operativo relevante.

## 4. Correlación con eventos

Se consideran eventos a ±12 h del inicio del cambio o dentro de su ventana.

| Evento | ¿Explica el cambio? |
|---|---|
| `SCHEDULED_OUTAGE` | Sí, si es una **caída que se recupera**. Si la descripción indica duración ("12 hours"), se compara con la observada |
| `OPERATIONAL_CHANGE` | Sí (cambio de nivel operativo) |
| `DATA_QUALITY` | No explica un cambio de consumo; **refuerza** un hallazgo de calidad de datos |
| `UNKNOWN` / otros | **Nunca.** Un registro "No operational event reported" *confirma* que no hay causa operativa |

## 5. Clasificación

```
¿≥ 2 señales de calidad de datos y el problema NO cae dentro de un cambio
 eléctricamente coherente (corriente ∝ consumo)?        → DATA_QUALITY
Para cada cambio de nivel:
  ¿evento que lo explica?
    SCHEDULED_OUTAGE                                     → FALSE_POSITIVE
    OPERATIONAL_CHANGE                                   → EXPLAINABLE_ANOMALY
  si no                                                  → REAL_ANOMALY
Picos aislados sin explicación                           → REAL_ANOMALY (Media/Baja)
```

### Severidad

| Tipo | Regla |
|---|---|
| Real | **Alta** si \|desv\| ≥ 50%, o si ≥ 25% con corroboración eléctrica en 2 variables. Si no, Media |
| Calidad de datos | **Alta** si afecta ≥ 10% de las lecturas o hay voltaje fuera de banda. Si no, Media |
| Explicable | **Media** si \|desv\| ≥ 25%. Si no, Baja |
| Falso positivo | Baja |

### Confianza

```
confianza = 0,40 + 0,25·magnitud + 0,25·corroboración + 0,10·contexto   (tope 0,98)
```

- **Magnitud:** \|desviación\| (con tope 1). En calidad de datos: fracción de lecturas afectadas × 2.
- **Corroboración:** proporción de chequeos independientes que respaldan la clasificación (persistencia, corriente coherente, cambio eléctrico, ausencia de problemas de datos, alineación y duración del evento…).
- **Contexto:** claridad de los eventos. Alineación temporal para los explicables. 1,0 si un registro confirma que no hubo evento operativo.

El tope en 0,98 es deliberado: el motor nunca afirma certeza total.

### Prioridad (0–100)

```
prioridad = severidad (Alta 60 · Media 35 · Baja 10)
          + tipo (Real 25 · Calidad 15 · Explicable 5 · Falso positivo 0)
          + 10 · magnitud normalizada
          + 5 si sigue activa en las últimas 24 h
```

## 6. Explicación y recomendación

`backend/internal/ai`:

- **Plantillas** (siempre disponibles): cada número del texto sale de la evidencia.
- **LLM** (Ollama o API compatible con OpenAI): recibe **solo** la evidencia en JSON y el borrador de las plantillas. Salida con esquema JSON estricto (`reason`, `recommended_action`).
- **Guardrails:** textos vacíos o demasiado largos se rechazan, y también cualquier porcentaje que no esté en la evidencia (±1,5 puntos). Ante un rechazo, un error o un timeout se usan las plantillas.
- La **lista de evidencia** siempre es la determinista del motor, nunca la del LLM.

## 7. Resultado en el dataset

| Medidor | Hallazgo | Evidencia clave |
|---|---|---|
| M-109 | Real · Alta · 0,98 | +110,4% desde el 12-sep 14:00, persistente. Corriente ×2,10 con consumo ×2,11. FP de 0,94 a 0,74. El registro `UNKNOWN` no explica el cambio |
| M-112 | Calidad de datos · Alta · 0,92 | 16 lecturas con V fuera de banda (201,6–241,2 V), 12 con potencia incoherente, consumo estable (+0,5%). Evento `DATA_QUALITY` |
| M-104 | Explicable · Media · 0,87 | +46,5% desde el 11-sep 00:00, alineado 0 h con "New production line activated" |
| M-106 | Falso positivo · Baja · 0,95 | −79,9% durante exactamente 12 h el 8-sep. Coincide con la parada programada de 12 h y se recupera |

Los ocho medidores restantes no generan hallazgos. `TestDataset_*` fija todo lo anterior.
