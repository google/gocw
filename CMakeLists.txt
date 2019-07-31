cmake_minimum_required (VERSION 3.5)
project ("chipwhisperer" C)

# Set default build type.
if(NOT CMAKE_BUILD_TYPE)
  message(STATUS "Setting build type to 'MinSizeRel' as none was specified.")
  set(CMAKE_BUILD_TYPE "MinSizeRel" CACHE STRING
      "Choose the type of build, options are: Debug Release RelWithDebInfo MinSizeRel."
      FORCE)
endif()

if(DEFINED ENV{PLATFORM})
  set(PLATFORM $ENV{PLATFORM})
endif()

if(NOT PLATFORM)
  message(STATUS "Setting PLATFORM to CWLITEXMEGA")
  set(PLATFORM "CWLITEXMEGA")
endif()

set(VALID_PLATFORMS CWLITEXMEGA CWLITEARM)
if(NOT PLATFORM IN_LIST VALID_PLATFORMS)
  message(FATAL_ERROR "Platform must be one of ${VALID_PLATFORMS}")
endif()

if (PLATFORM STREQUAL "CWLITEXMEGA")
  set(CMAKE_C_COMPILER  "avr-gcc")
  set(CMAKE_OBJCOPY  "avr-objcopy")
  set(CMAKE_C_FLAGS   "${CMAKE_C_FLAGS} -mmcu=atxmega128d3")
  add_definitions(-DHAL_TYPE=HAL_xmega -DPLATFORM=CWLITEXMEGA -DF_CPU=7372800UL)
elseif (PLATFORM STREQUAL "CWLITEARM")
  set(CMAKE_C_COMPILER  "arm-none-eabi-gcc")
  set(CMAKE_OBJCOPY  "arm-none-eabi-objcopy")
  set(CMAKE_C_FLAGS "${CMAKE_C_FLAGS} -mcpu=cortex-m4 -mthumb -mfloat-abi=hard -mfpu=fpv4-sp-d16 -fmessage-length=0 -ffunction-sections -funsigned-char -funsigned-bitfields -fshort-enums -Wall -Wstrict-prototypes -gdwarf-2")
  add_definitions(-DNO_EXTRA_OPTS -DPLATFORM=CWLITEARM -DHAL_TYPE=HAL_stm32f3 -DSTM32F303xC -DSTM32F3 -DSTM32 -DF_CPU=7372800UL)
  set(CMAKE_EXE_LINKER_FLAGS "${CMAKE_EXE_LINKER_FLAGS} -T ${CMAKE_SOURCE_DIR}/third_party/chipwhisperer/firmware/hal/stm32f3/LinkerScript.ld --specs=nano.specs -Wl,--gc-sections -lm")
endif()

include(third_party/cryptoc.cmake)
include(third_party/tiny_aes_c.cmake)
add_subdirectory(firmware)

