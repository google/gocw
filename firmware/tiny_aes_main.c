/*
 * Copyright 2019 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#include <stdint.h>
#include <stdlib.h>

#include "aes.h"
#include "hal.h"
#include "simpleserial.h"

static struct AES_ctx ctx = {};

uint8_t get_key(uint8_t *k) {
  AES_init_ctx(&ctx, k);
  return 0x00;
}

uint8_t get_pt(uint8_t *pt) {
  trigger_high();
  AES_ECB_encrypt(&ctx, pt);
  trigger_low();
  simpleserial_put('r', 16, pt);
  return 0x00;
}

uint8_t reset(uint8_t *x) { return 0x00; }

int main(void) {
  platform_init();
  init_uart();
  trigger_setup();

  simpleserial_init();
  simpleserial_addcmd('k', 16, get_key);
  simpleserial_addcmd('p', 16, get_pt);
  simpleserial_addcmd('x', 0, reset);
  while (1)
    simpleserial_get();
}
