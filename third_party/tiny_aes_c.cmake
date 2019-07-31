add_library(tiny_aes_c
  ${CMAKE_SOURCE_DIR}/third_party/tiny-AES-c/aes.c
)
target_include_directories(tiny_aes_c
  PUBLIC
  ${CMAKE_SOURCE_DIR}/third_party/tiny-AES-c
)
