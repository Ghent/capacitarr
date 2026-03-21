/**
 * ECharts Nuxt plugin — registers vue-echarts globally with tree-shaken modules.
 * Only imports the chart types and components actually used by the dashboard.
 */
import { use } from 'echarts/core';
import { CanvasRenderer } from 'echarts/renderers';
import { BarChart, LineChart, GaugeChart } from 'echarts/charts';
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  VisualMapComponent,
  MarkLineComponent,
} from 'echarts/components';
import VChart from 'vue-echarts';

// Register only the modules we use (tree-shaking)
use([
  CanvasRenderer,
  BarChart,
  LineChart,
  GaugeChart,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  VisualMapComponent,
  MarkLineComponent,
]);

export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.vueApp.component('VChart', VChart);
});
