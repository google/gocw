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

#include "cryptoc/p256.h"
#include "hal.h"
#include "simpleserial.h"

static p256_int k = {};

uint8_t get_key(uint8_t *key) {
  p256_from_bin(key, &k);
  return 0x00;
}

uint8_t get_pt(uint8_t *pt) {
  p256_int in_x;
  p256_int in_y;
  p256_int out_x;
  p256_int out_y;

  trigger_high();

  p256_init(&in_x);
  p256_init(&in_y);
  p256_init(&out_x);
  p256_init(&out_y);

  p256_from_bin(&pt[0], &in_x);
  p256_from_bin(&pt[P256_NBYTES], &in_y);

  if (p256_is_valid_point(&in_x, &in_y)) {
    p256_point_mul(&k, &in_x, &in_y, &out_x, &out_y);
  }

  p256_to_bin(&out_x, &pt[0]);
  p256_to_bin(&out_y, &pt[P256_NBYTES]);

  trigger_low();
  simpleserial_put('r', 2 * P256_NBYTES, pt);
  return 0x00;
}

uint8_t reset(uint8_t *x) { return 0x00; }

int main(void) {
  platform_init();
  init_uart();
  trigger_setup();

  p256_init(&k);
  simpleserial_init();
  simpleserial_addcmd('k', P256_NBYTES, get_key);
  simpleserial_addcmd('p', 2 * P256_NBYTES, get_pt);
  simpleserial_addcmd('x', 0, reset);
  while (1)
    simpleserial_get();
}
