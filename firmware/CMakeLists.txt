set(CW_FW_DIR ${CMAKE_SOURCE_DIR}/third_party/chipwhisperer/firmware)
set(HAL_SOURCES)
set(HAL_INCLUDES
  ${CW_FW_DIR}/hal
)

# Compile firmware and create load file for flash.
function (build_firmware name src)
  add_executable(${name}
    ${src}
  )
  target_link_libraries(${name}
    simpleserial
    hal
    tiny_aes_c
    cryptoc
  )
  add_custom_command(
    OUTPUT ${CMAKE_CURRENT_BINARY_DIR}/${name}.hex
    COMMAND ${CMAKE_OBJCOPY} -O ihex -R .eeprom -R .fuse -R .lock -R .signature $<TARGET_FILE:${name}> ${CMAKE_CURRENT_BINARY_DIR}/${name}.hex
    DEPENDS ${name}
  )
  add_custom_target(${name}_hex ALL
    DEPENDS
    ${CMAKE_CURRENT_BINARY_DIR}/${name}.hex
  )
endfunction()

if (PLATFORM STREQUAL "CWLITEXMEGA")
  # HAL sources
  list(APPEND HAL_SOURCES
    ${CW_FW_DIR}/hal/xmega/uart.c
    ${CW_FW_DIR}/hal/xmega/usart_driver.c
    ${CW_FW_DIR}/hal/xmega/xmega_hal.c
  )
elseif (PLATFORM STREQUAL "CWLITEARM")
  # HAL sources
  set_property(SOURCE ${CW_FW_DIR}/hal/stm32f3/stm32f3_startup.S PROPERTY LANGUAGE C)

  list(APPEND HAL_SOURCES
    ${CW_FW_DIR}/hal/stm32f3/stm32f3_hal.c
    ${CW_FW_DIR}/hal/stm32f3/stm32f3_hal_lowlevel.c
    ${CW_FW_DIR}/hal/stm32f3/stm32f3_sysmem.c
    ${CW_FW_DIR}/hal/stm32f3/stm32f3_startup.S
  )
  list(APPEND HAL_INCLUDES
    ${CW_FW_DIR}/hal/stm32f3
    ${CW_FW_DIR}/hal/stm32f3/CMSIS
    ${CW_FW_DIR}/hal/stm32f3/CMSIS/core
    ${CW_FW_DIR}/hal/stm32f3/CMSIS/device
    ${CW_FW_DIR}/hal/stm32f3/Legacy
  )
endif()

# Simple serial version
add_definitions(-DSS_VER=SS_VER_1_1)

add_library(hal
  ${HAL_SOURCES}
)
target_include_directories(hal
  PUBLIC
  ${HAL_INCLUDES}
)

add_library(simpleserial
  ${CW_FW_DIR}/simpleserial/simpleserial.c
)
target_include_directories(simpleserial
  PRIVATE
  ${CW_FW_DIR}/hal
  PUBLIC
  ${CW_FW_DIR}/simpleserial
)

build_firmware(inc_plaintext inc_plaintext_main.c)
build_firmware(tiny_aes tiny_aes_main.c)
build_firmware(cryptoc_ecdh cryptoc_ecdh_main.c)
