<script setup>
import DefaultLayout from './layouts/DefaultLayout.vue'
</script>

<template>
  <DefaultLayout>
    <router-view />
  </DefaultLayout>
</template>

<style>
:root {
  color-scheme: light;
}

body {
  margin: 0;
  min-width: 320px;
  background-color: #f8f7f3;
}

button,
input,
textarea {
  font: inherit;
}
</style>
