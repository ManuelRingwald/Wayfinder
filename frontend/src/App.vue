<template>
  <!-- WF2-32: App.vue is now a thin shell. <v-app> provides the Vuetify layout
       root; <router-view> swaps the whole view between the ASD scope ('/') and the
       admin dashboard ('/admin'). No <keep-alive>: leaving the ASD must fully
       unmount the map, not park it in the background. -->
  <v-app>
    <router-view />
  </v-app>
</template>

<script setup>
</script>
