/**
 * ECharts Nuxt plugin — registers vue-echarts globally with tree-shaken modules.
 * Only imports the chart types and components actually used by the analytics dashboards.
 */
import { use } from 'echarts/core';
import { CanvasRenderer } from 'echarts/renderers';
import { PieChart, BarChart, LineChart, TreemapChart } from 'echarts/charts';
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  VisualMapComponent,
  DataZoomComponent,
} from 'echarts/components';
import VChart from 'vue-echarts';

// Register only the modules we use (tree-shaking)
use([
  CanvasRenderer,
  PieChart,
  BarChart,
  LineChart,
  TreemapChart,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
  VisualMapComponent,
  DataZoomComponent,
]);

export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.vueApp.component('VChart', VChart);
});
